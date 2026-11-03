package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/pricing"
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

	// Everything the prices need, loaded once for every room type and plan: a handful of
	// queries per search however many rooms and plans there are (it was 3-4 per plan).
	today := s.today(st)
	sp, err := s.loadSearchPricing(ctx, req, today)
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
			if err := attachQuotes(&opt, req, st, today, sp); err != nil {
				return nil, err
			}
		}
		out = append(out, opt)
	}
	return out, nil
}

// searchPricing is what pricing every plan of a search needs, loaded in one go.
type searchPricing struct {
	plans map[uuid.UUID][]searchPlan    // by room type, in display order
	rates map[uuid.UUID][]pricing.Night // by plan, in date order
	typed *typedCode
	autos []autoOffer
}

type searchPlan struct {
	PlanQuote
	info planInfo
}

// loadSearchPricing reads every active plan, its nightly rates for the stay, the typed code
// and the live automatic offers: four queries, whatever the number of rooms and plans.
func (s *Service) loadSearchPricing(ctx context.Context, req SearchRequest, today time.Time) (searchPricing, error) {
	sp := searchPricing{plans: map[uuid.UUID][]searchPlan{}, rates: map[uuid.UUID][]pricing.Night{}}
	rows, err := s.pool.Query(ctx, `SELECT rp.room_type_id, rp.id, rp.code, rp.name, rp.refundable, rp.min_nights,
			rt.base_occupancy, rt.max_adults, rt.max_children, rt.extra_adult_paise
		FROM rate_plans rp JOIN room_types rt ON rt.id = rp.room_type_id
		WHERE rp.is_active AND rt.is_active ORDER BY rp.sort`)
	if err != nil {
		return sp, err
	}
	var planIDs []uuid.UUID
	for rows.Next() {
		var roomTypeID uuid.UUID
		var p searchPlan
		if err := rows.Scan(&roomTypeID, &p.PlanID, &p.Code, &p.Name, &p.Refundable, &p.info.minNights,
			&p.info.room.BaseOccupancy, &p.info.room.MaxAdults, &p.info.room.MaxChildren, &p.info.room.ExtraAdultPaise); err != nil {
			rows.Close()
			return sp, err
		}
		sp.plans[roomTypeID] = append(sp.plans[roomTypeID], p)
		planIDs = append(planIDs, p.PlanID)
	}
	if err := rows.Err(); err != nil {
		return sp, err
	}

	rows, err = s.pool.Query(ctx, `SELECT rate_plan_id, date, price_paise FROM night_rates
		WHERE rate_plan_id = ANY($1) AND date >= $2 AND date < $3 ORDER BY rate_plan_id, date`, planIDs, req.CheckIn, req.CheckOut)
	if err != nil {
		return sp, err
	}
	for rows.Next() {
		var planID uuid.UUID
		var n pricing.Night
		if err := rows.Scan(&planID, &n.Date, &n.PricePaise); err != nil {
			rows.Close()
			return sp, err
		}
		sp.rates[planID] = append(sp.rates[planID], n)
	}
	if err := rows.Err(); err != nil {
		return sp, err
	}

	if sp.typed, err = loadTyped(ctx, s.pool, req.PromoCode); err != nil {
		return sp, err
	}
	sp.autos, err = loadAutoOffers(ctx, s.pool, today)
	return sp, err
}

// attachQuotes prices every plan of one room type from what the search loaded, in memory.
func attachQuotes(opt *Option, req SearchRequest, st settings, today time.Time, sp searchPricing) error {
	for _, p := range sp.plans[opt.RoomTypeID] {
		hr := HoldRequest{RoomTypeID: opt.RoomTypeID, RatePlanID: p.PlanID, Stay: req.Stay, Party: req.Party, PromoCode: req.PromoCode}
		q, _, err := priceStay(sp.rates[p.PlanID], p.info, hr, st, today, sp.typed, sp.autos)
		var pe *PromoError
		if errors.As(err, &pe) {
			// Show prices anyway; tell the guest why the code didn't apply.
			opt.PromoError = pe.Error()
			q, _, err = priceStay(sp.rates[p.PlanID], p.info, hr, st, today, nil, sp.autos)
		}
		var re *RestrictionError
		if errors.As(err, &re) {
			continue // e.g. a non-refundable plan with a 2-night minimum on a 1-night stay
		}
		if err != nil {
			return err
		}
		p.Quote = q
		opt.Plans = append(opt.Plans, p.PlanQuote)
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

// LockNights locks a range of nights for a writer outside this package (rate edits), in
// the same date order every other writer uses, so it can't deadlock with a booking.
func LockNights(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, nights domain.Stay) error {
	_, err := lockNights(ctx, tx, roomTypeID, nights)
	return err
}

// CheckAllotment refuses an allotment below what is already sold or held on any night in
// the range that applies (nil applies = every night), inside the caller's transaction.
// The rows are locked first, like every writer.
func CheckAllotment(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, nights domain.Stay, newAllotment int, applies func(time.Time) bool) error {
	locked, err := lockNights(ctx, tx, roomTypeID, nights)
	if err != nil {
		return err
	}
	if err := countHolds(ctx, tx, roomTypeID, locked, uuid.Nil); err != nil {
		return err
	}
	for _, n := range locked {
		if applies != nil && !applies(n.date) {
			continue
		}
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
