package admin

import (
	"context"
	"encoding/json"
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
