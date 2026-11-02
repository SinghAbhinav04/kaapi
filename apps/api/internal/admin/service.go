// Package admin is the staff side of rates and inventory: the grid of prices and
// allotments, and bulk edits to it. Every edit locks the nights it touches through
// package inventory and is written to the audit log.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/pricing"
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Grid returns every room type's nights in the range, priced on one plan.
func (s *Service) Grid(ctx context.Context, stay domain.Stay, planCode string) (Grid, error) {
	if n := stay.Nights(); n < 1 || n > maxGridDays {
		return Grid{}, apperr.Validation(map[string]string{"to": fmt.Sprintf("must be 1 to %d nights after from", maxGridDays)})
	}
	rows, err := s.pool.Query(ctx, `
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
		return Grid{}, err
	}
	defer rows.Close()
	g := Grid{Plan: planCode, Stay: stay}
	for rows.Next() {
		var id uuid.UUID
		var slug, name string
		var keys int
		var n Night
		if err := rows.Scan(&id, &slug, &name, &keys, &n.Date, &n.PricePaise, &n.Allotment, &n.Sold, &n.Held,
			&n.StopSell, &n.StopSellReason, &n.MinStay, &n.NoArrival); err != nil {
			return Grid{}, err
		}
		n.Left = max(n.Allotment-n.Sold-n.Held, 0)
		if len(g.Rows) == 0 || g.Rows[len(g.Rows)-1].RoomTypeID != id {
			g.Rows = append(g.Rows, Row{RoomTypeID: id, Slug: slug, Name: name, TotalKeys: keys})
		}
		r := &g.Rows[len(g.Rows)-1]
		r.Nights = append(r.Nights, n)
	}
	if err := rows.Err(); err != nil {
		return Grid{}, err
	}
	if len(g.Rows) == 0 {
		return Grid{}, ErrPlanNotFound
	}
	return g, nil
}

// Apply makes a bulk edit and returns how many nights it changed.
func (s *Service) Apply(ctx context.Context, actor uuid.UUID, c Change) (int, error) {
	if err := validate(&c); err != nil {
		return 0, err
	}
	applies := func(d time.Time) bool { return len(c.Weekdays) == 0 || slices.Contains(c.Weekdays, d.Weekday()) }
	var dates []time.Time
	for d := c.CheckIn; d.Before(c.CheckOut); d = d.AddDate(0, 0, 1) {
		if applies(d) {
			dates = append(dates, d)
		}
	}
	if len(dates) == 0 {
		return 0, ErrNoMatchingNight
	}

	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var keys int
		err := tx.QueryRow(ctx, `SELECT total_keys FROM room_types WHERE id = $1`, c.RoomTypeID).Scan(&keys)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoomNotFound
		}
		if err != nil {
			return err
		}
		if c.Allotment != nil {
			if *c.Allotment > keys {
				return apperr.Validation(map[string]string{"allotment": fmt.Sprintf("can't be more than the %d keys of this room type", keys)})
			}
			// Locks the nights and refuses to go below rooms already sold or on hold.
			if err := inventory.CheckAllotment(ctx, tx, c.RoomTypeID, c.Stay, *c.Allotment, applies); err != nil {
				return err
			}
		} else if err := inventory.LockNights(ctx, tx, c.RoomTypeID, c.Stay); err != nil {
			return err
		}

		if c.TouchesInventory() {
			if _, err := tx.Exec(ctx, `UPDATE night_inventory SET
				allotment = coalesce($3, allotment),
				stop_sell = coalesce($4, stop_sell),
				stop_sell_reason = CASE WHEN $4::boolean IS NULL THEN stop_sell_reason WHEN $4 THEN $5 ELSE NULL END,
				min_stay = coalesce($6, min_stay),
				closed_to_arrival = coalesce($7, closed_to_arrival),
				updated_by = $8, updated_at = now()
				WHERE room_type_id = $1 AND date = ANY($2)`,
				c.RoomTypeID, dates, c.Allotment, c.StopSell, c.StopSellReason, c.MinStay, c.NoArrival, actor); err != nil {
				return err
			}
		}
		if c.RoomOnlyPricePaise != nil {
			if err := s.reprice(ctx, tx, c.RoomTypeID, dates, *c.RoomOnlyPricePaise, actor); err != nil {
				return err
			}
		}
		after, _ := json.Marshal(auditChange(c, len(dates)))
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs (id, actor_user_id, action, entity, entity_id, after) VALUES ($1,$2,$3,$4,$5,$6)`,
			idgen.New(), actor, auditActionRates, auditEntityRoom, c.RoomTypeID.String(), after)
		return err
	})
	if err != nil {
		return 0, err
	}
	return len(dates), nil
}

// reprice sets the room-only price and derives every other plan from it.
func (s *Service) reprice(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, dates []time.Time, roomOnly int64, actor uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT id, code, add_paise_per_night, discount_bps FROM rate_plans WHERE room_type_id = $1`, roomTypeID)
	if err != nil {
		return err
	}
	type plan struct {
		id       uuid.UUID
		code     string
		add, bps int64
	}
	plans, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (plan, error) {
		var p plan
		return p, r.Scan(&p.id, &p.code, &p.add, &p.bps)
	})
	if err != nil {
		return err
	}
	for _, p := range plans {
		price := roomOnly
		if p.code != RoomOnlyPlan {
			price = pricing.DerivePlanPrice(roomOnly, p.add, p.bps, planRoundingPaise)
		}
		if _, err := tx.Exec(ctx, `UPDATE night_rates SET price_paise = $3, updated_by = $4, updated_at = now()
			WHERE rate_plan_id = $1 AND date = ANY($2)`, p.id, dates, price, actor); err != nil {
			return err
		}
	}
	return nil
}

func validate(c *Change) error {
	f := map[string]string{}
	if c.RoomTypeID == uuid.Nil {
		f["room_type_id"] = "is required"
	}
	if n := c.Nights(); n < 1 || n > maxEditDays {
		f["to"] = fmt.Sprintf("must be 1 to %d nights after from", maxEditDays)
	}
	if p := c.RoomOnlyPricePaise; p != nil && (*p <= 0 || *p > maxPricePaise) {
		f["room_only_price_paise"] = "must be a positive price per night"
	}
	if a := c.Allotment; a != nil && *a < 0 {
		f["allotment"] = "can't be negative"
	}
	if m := c.MinStay; m != nil && (*m < 1 || *m > maxMinStay) {
		f["min_stay"] = fmt.Sprintf("must be 1 to %d nights", maxMinStay)
	}
	if c.StopSellReason != nil {
		r := strings.TrimSpace(*c.StopSellReason)
		c.StopSellReason = &r
		if len([]rune(r)) > stopSellReasonMax {
			f["stop_sell_reason"] = "is too long"
		}
	}
	if len(f) > 0 {
		return apperr.Validation(f)
	}
	if !c.TouchesPrice() && !c.TouchesInventory() {
		return ErrNothingToChange
	}
	return nil
}

func auditChange(c Change, nights int) map[string]any {
	days := make([]string, len(c.Weekdays))
	for i, w := range c.Weekdays {
		days[i] = w.String()[:3]
	}
	return map[string]any{"from": c.CheckIn.Format(time.DateOnly), "to": c.CheckOut.Format(time.DateOnly), "weekdays": days,
		"nights": nights, "room_only_price_paise": c.RoomOnlyPricePaise, "allotment": c.Allotment, "stop_sell": c.StopSell,
		"stop_sell_reason": c.StopSellReason, "min_stay": c.MinStay, "no_arrival": c.NoArrival}
}
