package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
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
