package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
)

// Search answers "what can I book for these dates?" for every room type, with prices for
// every plan. It never locks anything; the hold re-checks under lock.
func (s *Service) Search(ctx context.Context, req SearchRequest) ([]Option, error) {
	st, err := loadSettings(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	if err := validateStay(req.Stay, req.Party, st, s.today(st)); err != nil {
		return nil, err
	}

	type roomRow struct {
		id                              uuid.UUID
		slug, name                      string
		baseOcc, maxAdults, maxChildren int
	}
	rows, err := s.pool.Query(ctx, `SELECT id, slug, name, base_occupancy, max_adults, max_children FROM room_types WHERE is_active ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	rooms, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (roomRow, error) {
		var x roomRow
		return x, r.Scan(&x.id, &x.slug, &x.name, &x.baseOcc, &x.maxAdults, &x.maxChildren)
	})
	if err != nil {
		return nil, err
	}

	avail, err := s.nightsByRoom(ctx, req.CheckIn, req.CheckOut)
	if err != nil {
		return nil, err
	}

	nights := req.Nights()
	out := make([]Option, 0, len(rooms))
	for _, rt := range rooms {
		opt := Option{RoomTypeID: rt.id, Slug: rt.slug, Name: rt.name, Status: Available}
		ns := avail[rt.id]
		switch {
		case len(ns) != nights:
			opt.Status = NotYetOnSale
		case req.Adults > rt.maxAdults*req.Rooms || req.Children > rt.maxChildren*req.Rooms:
			opt.Status = TooManyGuests
		default:
			opt.Left = ns[0].left
			for _, n := range ns {
				opt.Left = min(opt.Left, n.left)
				if n.left < req.Rooms {
					opt.SoldOutNights = append(opt.SoldOutNights, n.date)
				}
			}
			opt.Left = max(opt.Left, 0)
			opt.MinStay = ns[0].minStay
			switch {
			case anyStopSell(ns):
				opt.Status, opt.Reason = Restricted, "Not available on these dates"
			case ns[0].cta:
				opt.Status, opt.Reason = Restricted, "No arrivals on this date"
			case nights < ns[0].minStay:
				opt.Status, opt.Reason = Restricted, "Minimum stay applies"
			case len(opt.SoldOutNights) > 0:
				opt.Status = SoldOut
			}
		}
		if opt.Status == Available {
			if err := s.attachQuotes(ctx, &opt, req, st); err != nil {
				return nil, err
			}
		}
		out = append(out, opt)
	}
	return out, nil
}

func (s *Service) attachQuotes(ctx context.Context, opt *Option, req SearchRequest, st settings) error {
	rows, err := s.pool.Query(ctx, `SELECT id, code, name, refundable FROM rate_plans WHERE room_type_id = $1 AND is_active ORDER BY sort`, opt.RoomTypeID)
	if err != nil {
		return err
	}
	plans, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (PlanQuote, error) {
		var p PlanQuote
		return p, r.Scan(&p.PlanID, &p.Code, &p.Name, &p.Refundable)
	})
	if err != nil {
		return err
	}
	for _, p := range plans {
		hr := HoldRequest{RoomTypeID: opt.RoomTypeID, RatePlanID: p.PlanID, Stay: req.Stay, Party: req.Party, PromoCode: req.PromoCode}
		q, _, err := quoteStay(ctx, s.pool, hr, st, s.today(st))
		var pe *PromoError
		if errors.As(err, &pe) {
			// Show prices anyway; tell the guest why the code didn't apply.
			opt.PromoError = pe.Error()
			hr.PromoCode = ""
			q, _, err = quoteStay(ctx, s.pool, hr, st, s.today(st))
		}
		var re *RestrictionError
		if errors.As(err, &re) {
			continue // e.g. a non-refundable plan with a 2-night minimum on a 1-night stay
		}
		if err != nil {
			return err
		}
		p.Quote = q
		opt.Plans = append(opt.Plans, p)
	}
	return nil
}

// nightsByRoom returns rooms left per night (allotment − sold − live holds) for every room type.
func (s *Service) nightsByRoom(ctx context.Context, from, to time.Time) (map[uuid.UUID][]nightAvail, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ni.room_type_id, ni.date,
		       ni.allotment - ni.sold - coalesce(h.held, 0) AS left,
		       ni.min_stay, ni.stop_sell, ni.closed_to_arrival
		FROM night_inventory ni
		LEFT JOIN LATERAL (
			SELECT sum(rooms)::int AS held FROM holds
			WHERE room_type_id = ni.room_type_id AND status = $3 AND expires_at > now()
			  AND check_in <= ni.date AND check_out > ni.date
		) h ON true
		WHERE ni.date >= $1 AND ni.date < $2
		ORDER BY ni.room_type_id, ni.date`, from, to, domain.HoldActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]nightAvail{}
	for rows.Next() {
		var id uuid.UUID
		var n nightAvail
		if err := rows.Scan(&id, &n.date, &n.left, &n.minStay, &n.stopSell, &n.cta); err != nil {
			return nil, err
		}
		out[id] = append(out[id], n)
	}
	return out, rows.Err()
}

// Alternatives finds nearby check-in dates (within a week either side) where the same
// length of stay is fully available, nearest first.
func (s *Service) Alternatives(ctx context.Context, roomTypeID uuid.UUID, checkIn time.Time, nights, rooms int) ([]time.Time, error) {
	st, err := loadSettings(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	today := s.today(st)
	from := checkIn.AddDate(0, 0, -alternativesWindowDays)
	if from.Before(today) {
		from = today
	}
	all, err := s.nightsByRoom(ctx, from, checkIn.AddDate(0, 0, alternativesWindowDays+nights))
	if err != nil {
		return nil, err
	}
	ns := all[roomTypeID]
	byDate := make(map[string]nightAvail, len(ns)) // string keys: time.Time keys compare locations too
	for _, n := range ns {
		byDate[n.date.Format(time.DateOnly)] = n
	}
	ok := func(start time.Time) bool {
		first, found := byDate[start.Format(time.DateOnly)]
		if !found || first.cta || nights < first.minStay {
			return false
		}
		for i := 0; i < nights; i++ {
			n, found := byDate[start.AddDate(0, 0, i).Format(time.DateOnly)]
			if !found || n.stopSell || n.left < rooms {
				return false
			}
		}
		return true
	}
	var out []time.Time
	for delta := 1; delta <= alternativesWindowDays && len(out) < maxAlternatives; delta++ {
		for _, d := range []time.Time{checkIn.AddDate(0, 0, -delta), checkIn.AddDate(0, 0, delta)} {
			if !d.Before(today) && ok(d) && len(out) < maxAlternatives {
				out = append(out, d)
			}
		}
	}
	return out, nil
}

// CheckAllotment refuses an allotment below what is already sold or held on any night in
// the range, inside the caller's transaction (the rows are locked first, like every writer).
func CheckAllotment(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, nights domain.Stay, newAllotment int) error {
	locked, err := lockNights(ctx, tx, roomTypeID, nights)
	if err != nil {
		return err
	}
	if err := countHolds(ctx, tx, roomTypeID, locked, uuid.Nil); err != nil {
		return err
	}
	for _, n := range locked {
		if n.sold+n.held > newAllotment {
			return &AllotmentError{Date: n.date, Sold: n.sold, Held: n.held, RequestedTo: newAllotment}
		}
	}
	return nil
}

func anyStopSell(ns []nightAvail) bool {
	for _, n := range ns {
		if n.stopSell {
			return true
		}
	}
	return false
}
