package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/pricing"
)

// store is every query the hold engine runs. The overbooking rules (lock order, restriction
// and capacity checks) live in hold.go; these only read and write rows.
type store struct{}

// lockNights locks a room type's nights in [from, to) in date order. Locking in date order
// is what keeps concurrent holds on overlapping stays from deadlocking.
func (store) lockNights(ctx context.Context, q db.Querier, roomTypeID uuid.UUID, from, to time.Time) ([]night, error) {
	rows, err := q.Query(ctx, `SELECT date, allotment, sold, stop_sell, stop_sell_reason, min_stay, closed_to_arrival, closed_to_departure
		FROM night_inventory WHERE room_type_id = $1 AND date >= $2 AND date < $3
		ORDER BY date FOR UPDATE`, roomTypeID, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (night, error) {
		var n night
		return n, r.Scan(&n.date, &n.allotment, &n.sold, &n.stopSell, &n.stopSellReason, &n.minStay, &n.closedToArrival, &n.closedToDeparture)
	})
}

// departuresClosed lists the room types that take no departures on day, for a search.
func (store) departuresClosed(ctx context.Context, q Querier, day time.Time) (map[uuid.UUID]bool, error) {
	rows, err := q.Query(ctx, `SELECT room_type_id FROM night_inventory WHERE date = $1 AND closed_to_departure`, day)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	out := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}

// closedToDeparture reports the departure-day rule; false when that night isn't loaded.
func (store) closedToDeparture(ctx context.Context, q db.Querier, roomTypeID uuid.UUID, day time.Time) (bool, error) {
	var ctd bool
	err := q.QueryRow(ctx, `SELECT closed_to_departure FROM night_inventory WHERE room_type_id = $1 AND date = $2`, roomTypeID, day).Scan(&ctd)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ctd, err
}

// heldPerNight is the rooms held by live holds (active, not expired) on each night from
// first to last inclusive, ignoring one hold (uuid.Nil ignores none). Expired holds stop
// counting the moment they expire, with no job needed.
func (store) heldPerNight(ctx context.Context, q db.Querier, roomTypeID uuid.UUID, first, last time.Time, except uuid.UUID) ([]int, error) {
	rows, err := q.Query(ctx, `SELECT coalesce(sum(h.rooms), 0)::int
		FROM (SELECT gs::date AS d FROM generate_series($2::date, $3::date, interval '1 day') AS gs) g
		LEFT JOIN holds h ON h.room_type_id = $1 AND h.status = $5 AND h.expires_at > now()
		                 AND h.check_in <= g.d AND h.check_out > g.d AND h.id <> $4
		GROUP BY g.d ORDER BY g.d`, roomTypeID, first, last, except, domain.HoldActive)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int])
}

// insertHold stores a hold with its frozen quote and returns when it expires.
func (store) insertHold(ctx context.Context, q db.Querier, h Hold, req HoldRequest, tokenHash, quoteJSON []byte, promoID *uuid.UUID, minutes int) (time.Time, error) {
	var expires time.Time
	err := q.QueryRow(ctx, `INSERT INTO holds
		(id, room_type_id, rate_plan_id, check_in, check_out, rooms, adults, children, hold_token_hash, quote, promo_code_id, status, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now() + make_interval(mins => $13))
		RETURNING expires_at`,
		h.ID, req.RoomTypeID, req.RatePlanID, req.CheckIn, req.CheckOut, req.Rooms, req.Adults, req.Children,
		tokenHash, quoteJSON, promoID, domain.HoldActive, minutes).Scan(&expires)
	return expires, err
}

// holdTokenHash returns pgx.ErrNoRows for an unknown hold.
func (store) holdTokenHash(ctx context.Context, q db.Querier, id uuid.UUID) ([]byte, error) {
	var h []byte
	err := q.QueryRow(ctx, `SELECT hold_token_hash FROM holds WHERE id = $1`, id).Scan(&h)
	return h, err
}

// release reports false when the hold wasn't active.
func (store) release(ctx context.Context, q db.Querier, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE holds SET status = $2 WHERE id = $1 AND status = $3`, id, domain.HoldReleased, domain.HoldActive)
	return err == nil && tag.RowsAffected() == 1, err
}

// awaitingPayment reports whether a booking on this hold is waiting for its payment.
func (store) awaitingPayment(ctx context.Context, q db.Querier, holdID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM bookings WHERE hold_id = $1 AND status = $2)`,
		holdID, domain.BookingPendingPayment).Scan(&ok)
	return ok, err
}

// heldStay is a hold as confirming it needs, locked.
type heldStay struct {
	roomTypeID uuid.UUID
	stay       domain.Stay
	rooms      int
	status     domain.HoldStatus
	expired    bool
}

// lockHold returns pgx.ErrNoRows for an unknown hold.
func (store) lockHold(ctx context.Context, q db.Querier, id uuid.UUID) (heldStay, error) {
	var h heldStay
	err := q.QueryRow(ctx, `SELECT room_type_id, check_in, check_out, rooms, status, expires_at <= now()
		FROM holds WHERE id = $1 FOR UPDATE`, id).Scan(&h.roomTypeID, &h.stay.CheckIn, &h.stay.CheckOut, &h.rooms, &h.status, &h.expired)
	return h, err
}

func (store) setHoldStatus(ctx context.Context, q db.Querier, id uuid.UUID, status domain.HoldStatus) error {
	_, err := q.Exec(ctx, `UPDATE holds SET status = $2 WHERE id = $1`, id, status)
	return err
}

// sellNight counts rooms as sold on one night and records it against the booking. The
// CHECK (sold <= allotment) constraint is the backstop if the checks above were ever wrong.
func (store) sellNight(ctx context.Context, q db.Querier, roomTypeID, bookingID uuid.UUID, day time.Time, rooms int) error {
	if _, err := q.Exec(ctx, `UPDATE night_inventory SET sold = sold + $3, updated_at = now()
		WHERE room_type_id = $1 AND date = $2`, roomTypeID, day, rooms); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `INSERT INTO booking_nights (booking_id, date, room_type_id, rooms) VALUES ($1,$2,$3,$4)`,
		bookingID, day, roomTypeID, rooms)
	return err
}

// ---- search and pricing loaders

// searchRoom is a room type as the search lists it.
type searchRoom struct {
	id                              uuid.UUID
	slug, name                      string
	kind                            domain.RoomKind
	baseOcc, maxAdults, maxChildren int
}

func (store) searchRooms(ctx context.Context, q Querier) ([]searchRoom, error) {
	rows, err := q.Query(ctx, `SELECT id, slug, name, kind, base_occupancy, max_adults, max_children FROM room_types WHERE is_active ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (searchRoom, error) {
		var x searchRoom
		return x, r.Scan(&x.id, &x.slug, &x.name, &x.kind, &x.baseOcc, &x.maxAdults, &x.maxChildren)
	})
}

// activePlans is every plan on sale, by room type in display order, and their ids.
func (store) activePlans(ctx context.Context, q Querier) (map[uuid.UUID][]searchPlan, []uuid.UUID, error) {
	plans := map[uuid.UUID][]searchPlan{}
	rows, err := q.Query(ctx, `SELECT rp.room_type_id, rp.id, rp.code, rp.name, rp.refundable, rp.min_nights,
			rt.base_occupancy, rt.max_adults, rt.max_children, rt.extra_adult_paise
		FROM rate_plans rp JOIN room_types rt ON rt.id = rp.room_type_id
		WHERE rp.is_active AND rt.is_active ORDER BY rp.sort`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var roomTypeID uuid.UUID
		var p searchPlan
		if err := rows.Scan(&roomTypeID, &p.PlanID, &p.Code, &p.Name, &p.Refundable, &p.info.minNights,
			&p.info.room.BaseOccupancy, &p.info.room.MaxAdults, &p.info.room.MaxChildren, &p.info.room.ExtraAdultPaise); err != nil {
			return nil, nil, err
		}
		plans[roomTypeID] = append(plans[roomTypeID], p)
		ids = append(ids, p.PlanID)
	}
	return plans, ids, rows.Err()
}

// ratesFor is each plan's nightly prices for [from, to), in date order.
func (store) ratesFor(ctx context.Context, q Querier, planIDs []uuid.UUID, from, to time.Time) (map[uuid.UUID][]pricing.Night, error) {
	out := map[uuid.UUID][]pricing.Night{}
	rows, err := q.Query(ctx, `SELECT rate_plan_id, date, price_paise FROM night_rates
		WHERE rate_plan_id = ANY($1) AND date >= $2 AND date < $3 ORDER BY rate_plan_id, date`, planIDs, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var planID uuid.UUID
		var n pricing.Night
		if err := rows.Scan(&planID, &n.Date, &n.PricePaise); err != nil {
			return nil, err
		}
		out[planID] = append(out[planID], n)
	}
	return out, rows.Err()
}

// planFor is what pricing one plan needs; pgx.ErrNoRows when the plan isn't on sale for that room.
func (store) planFor(ctx context.Context, q Querier, roomTypeID, planID uuid.UUID) (planInfo, error) {
	var p planInfo
	err := q.QueryRow(ctx, `SELECT rt.base_occupancy, rt.max_adults, rt.max_children, rt.extra_adult_paise, rp.min_nights
		FROM rate_plans rp JOIN room_types rt ON rt.id = rp.room_type_id
		WHERE rp.id = $1 AND rt.id = $2 AND rp.is_active AND rt.is_active`, planID, roomTypeID).
		Scan(&p.room.BaseOccupancy, &p.room.MaxAdults, &p.room.MaxChildren, &p.room.ExtraAdultPaise, &p.minNights)
	return p, err
}

// nightRates is one plan's nightly prices for [from, to), in date order.
func (store) nightRates(ctx context.Context, q Querier, planID uuid.UUID, from, to time.Time) ([]pricing.Night, error) {
	rows, err := q.Query(ctx, `SELECT date, price_paise FROM night_rates
		WHERE rate_plan_id = $1 AND date >= $2 AND date < $3 ORDER BY date`, planID, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (pricing.Night, error) {
		var n pricing.Night
		return n, r.Scan(&n.Date, &n.PricePaise)
	})
}

// nightsByRoom returns rooms left per night (allotment − sold − live holds) for every room type.
func (store) nightsByRoom(ctx context.Context, q Querier, from, to time.Time) (map[uuid.UUID][]nightAvail, error) {
	rows, err := q.Query(ctx, `
		SELECT ni.room_type_id, ni.date,
		       ni.allotment - ni.sold - coalesce(h.held, 0) AS left,
		       ni.min_stay, ni.stop_sell, ni.closed_to_arrival, ni.closed_to_departure
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
		if err := rows.Scan(&id, &n.date, &n.left, &n.minStay, &n.stopSell, &n.cta, &n.ctd); err != nil {
			return nil, err
		}
		out[id] = append(out[id], n)
	}
	return out, rows.Err()
}

func loadSettings(ctx context.Context, q Querier) (settings, error) {
	var st settings
	var tz string
	var slabs []byte
	err := q.QueryRow(ctx, `SELECT timezone, hold_minutes, max_nights, max_rooms_per_booking, gst_slabs FROM property_settings`).
		Scan(&tz, &st.holdMinutes, &st.maxNights, &st.maxRooms, &slabs)
	if err != nil {
		return st, fmt.Errorf("load property settings: %w", err)
	}
	if st.location, err = time.LoadLocation(tz); err != nil {
		st.location = istFallback
	}
	return st, json.Unmarshal(slabs, &st.slabs)
}

// loadAutoOffers returns the automatic offers bookable today; Eligible then checks the stay.
func loadAutoOffers(ctx context.Context, q Querier, today time.Time) ([]autoOffer, error) {
	rows, err := q.Query(ctx, `SELECT id, code, type, value, stay_from, stay_to, book_from, book_to, min_nights, max_uses, used_count,
		coalesce(room_type_ids, '{}'), is_active FROM promo_codes
		WHERE auto_apply AND is_active AND book_from <= $1 AND book_to >= $1
		  AND (max_uses IS NULL OR used_count < max_uses)
		ORDER BY code`, today)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (autoOffer, error) {
		var a autoOffer
		err := r.Scan(&a.id, &a.rule.Code, &a.rule.Type, &a.rule.Value, &a.rule.StayFrom, &a.rule.StayTo, &a.rule.BookFrom, &a.rule.BookTo,
			&a.rule.MinNights, &a.rule.MaxUses, &a.rule.UsedCount, &a.rule.RoomTypeIDs, &a.rule.Active)
		return a, err
	})
}

func loadPromo(ctx context.Context, q Querier, code string) (pricing.PromoRule, uuid.UUID, error) {
	var r pricing.PromoRule
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id, code, type, value, stay_from, stay_to, book_from, book_to, min_nights, max_uses, used_count,
		coalesce(room_type_ids, '{}'), is_active FROM promo_codes WHERE code = $1`, code).
		Scan(&id, &r.Code, &r.Type, &r.Value, &r.StayFrom, &r.StayTo, &r.BookFrom, &r.BookTo, &r.MinNights, &r.MaxUses, &r.UsedCount,
			&r.RoomTypeIDs, &r.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, id, &PromoError{Err: errPromoUnknown}
	}
	return r, id, err
}
