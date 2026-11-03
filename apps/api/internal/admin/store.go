package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

// store is every query this package runs.
type store struct{}

// gridNight is one room type's night as the grid query returns it.
type gridNight struct {
	roomTypeID uuid.UUID
	slug, name string
	totalKeys  int
	night      Night
}

// planPricing is what deriving a plan's price from room-only needs.
type planPricing struct {
	id       uuid.UUID
	code     string
	add, bps int64
}

// grid is every active room type's nights in the stay, priced on one plan, with live holds.
func (store) grid(ctx context.Context, q db.Querier, stay domain.Stay, planCode string) ([]gridNight, error) {
	rows, err := q.Query(ctx, `
		SELECT rt.id, rt.slug, rt.name, rt.total_keys, ni.date, nr.price_paise, ni.allotment, ni.sold,
		       coalesce(h.held, 0), ni.stop_sell, ni.stop_sell_reason, ni.min_stay, ni.closed_to_arrival
		FROM room_types rt
		JOIN rate_plans rp ON rp.room_type_id = rt.id AND rp.code = $3
		JOIN night_inventory ni ON ni.room_type_id = rt.id AND ni.date >= $1 AND ni.date < $2
		JOIN night_rates nr ON nr.rate_plan_id = rp.id AND nr.date = ni.date
		LEFT JOIN LATERAL (
			SELECT sum(rooms)::int AS held FROM holds
			WHERE room_type_id = rt.id AND status = $4 AND expires_at > now() AND check_in <= ni.date AND check_out > ni.date
		) h ON true
		WHERE rt.is_active
		ORDER BY rt.sort, ni.date`, stay.CheckIn, stay.CheckOut, planCode, domain.HoldActive)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (gridNight, error) {
		var g gridNight
		n := &g.night
		return g, r.Scan(&g.roomTypeID, &g.slug, &g.name, &g.totalKeys, &n.Date, &n.PricePaise, &n.Allotment, &n.Sold, &n.Held,
			&n.StopSell, &n.StopSellReason, &n.MinStay, &n.NoArrival)
	})
}

// totalKeys returns pgx.ErrNoRows for an unknown room type.
func (store) totalKeys(ctx context.Context, q db.Querier, roomTypeID uuid.UUID) (int, error) {
	var keys int
	err := q.QueryRow(ctx, `SELECT total_keys FROM room_types WHERE id = $1`, roomTypeID).Scan(&keys)
	return keys, err
}

// updateInventory applies the inventory fields that are set; nil leaves a column as it is.
func (store) updateInventory(ctx context.Context, q db.Querier, c Change, dates []time.Time, actor uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE night_inventory SET
		allotment = coalesce($3, allotment),
		stop_sell = coalesce($4, stop_sell),
		stop_sell_reason = CASE WHEN $4::boolean IS NULL THEN stop_sell_reason WHEN $4 THEN $5 ELSE NULL END,
		min_stay = coalesce($6, min_stay),
		closed_to_arrival = coalesce($7, closed_to_arrival),
		updated_by = $8, updated_at = now()
		WHERE room_type_id = $1 AND date = ANY($2)`,
		c.RoomTypeID, dates, c.Allotment, c.StopSell, c.StopSellReason, c.MinStay, c.NoArrival, actor)
	return err
}

func (store) plans(ctx context.Context, q db.Querier, roomTypeID uuid.UUID) ([]planPricing, error) {
	rows, err := q.Query(ctx, `SELECT id, code, add_paise_per_night, discount_bps FROM rate_plans WHERE room_type_id = $1`, roomTypeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (planPricing, error) {
		var p planPricing
		return p, r.Scan(&p.id, &p.code, &p.add, &p.bps)
	})
}

func (store) setPrice(ctx context.Context, q db.Querier, planID uuid.UUID, dates []time.Time, price int64, actor uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE night_rates SET price_paise = $3, updated_by = $4, updated_at = now()
		WHERE rate_plan_id = $1 AND date = ANY($2)`, planID, dates, price, actor)
	return err
}

func (store) audit(ctx context.Context, q db.Querier, actor, roomTypeID uuid.UUID, after map[string]any) error {
	b, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_logs (id, actor_user_id, action, entity, entity_id, after) VALUES ($1,$2,$3,$4,$5,$6)`,
		idgen.New(), actor, auditActionRates, auditEntityRoom, roomTypeID.String(), b)
	return err
}

const offerColumns = `id, code, description, type, value, stay_from, stay_to, book_from, book_to, min_nights, max_uses, used_count,
	coalesce(room_type_ids, '{}'), auto_apply, is_active, created_at, updated_at`

func scanOffer(r pgx.Row) (Offer, error) {
	var o Offer
	return o, r.Scan(&o.ID, &o.Code, &o.Description, &o.Type, &o.Value, &o.StayFrom, &o.StayTo, &o.BookFrom, &o.BookTo,
		&o.MinNights, &o.MaxUses, &o.UsedCount, &o.RoomTypeIDs, &o.AutoApply, &o.Active, &o.CreatedAt, &o.UpdatedAt)
}

func (store) offers(ctx context.Context, q db.Querier) ([]Offer, error) {
	rows, err := q.Query(ctx, `SELECT `+offerColumns+` FROM promo_codes ORDER BY is_active DESC, book_to DESC, code`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Offer, error) { return scanOffer(r) })
}

func (store) offer(ctx context.Context, q db.Querier, id uuid.UUID) (Offer, error) {
	o, err := scanOffer(q.QueryRow(ctx, `SELECT `+offerColumns+` FROM promo_codes WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrOfferNotFound
	}
	return o, err
}

// roomIDs stores nil for "every room type", as the pricing code expects.
func roomIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func (store) insertOffer(ctx context.Context, q db.Querier, id uuid.UUID, in OfferInput) error {
	_, err := q.Exec(ctx, `INSERT INTO promo_codes (id, code, description, type, value, stay_from, stay_to, book_from, book_to,
		min_nights, max_uses, room_type_ids, auto_apply, is_active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		id, in.Code, in.Description, in.Type, in.Value, in.StayFrom, in.StayTo, in.BookFrom, in.BookTo,
		in.MinNights, in.MaxUses, roomIDs(in.RoomTypeIDs), in.AutoApply, in.Active)
	return err
}

func (store) updateOffer(ctx context.Context, q db.Querier, id uuid.UUID, in OfferInput) error {
	_, err := q.Exec(ctx, `UPDATE promo_codes SET code = $2, description = $3, type = $4, value = $5, stay_from = $6, stay_to = $7,
		book_from = $8, book_to = $9, min_nights = $10, max_uses = $11, room_type_ids = $12, auto_apply = $13, is_active = $14,
		updated_at = now() WHERE id = $1`,
		id, in.Code, in.Description, in.Type, in.Value, in.StayFrom, in.StayTo, in.BookFrom, in.BookTo,
		in.MinNights, in.MaxUses, roomIDs(in.RoomTypeIDs), in.AutoApply, in.Active)
	return err
}

// checkRooms refuses room type ids that don't exist (an array can't carry a foreign key).
func (store) checkRooms(ctx context.Context, q db.Querier, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	var n int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM room_types WHERE id = ANY($1)`, ids).Scan(&n); err != nil {
		return err
	}
	if n != len(ids) {
		return ErrOfferRoom
	}
	return nil
}

func (store) auditEntry(ctx context.Context, q db.Querier, actor uuid.UUID, action, entity, entityID string, after map[string]any) error {
	b, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_logs (id, actor_user_id, action, entity, entity_id, after) VALUES ($1,$2,$3,$4,$5,$6)`,
		idgen.New(), actor, action, entity, entityID, b)
	return err
}
