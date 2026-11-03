// Package admin is the staff side of rates and inventory: the grid of prices and
// allotments, and bulk edits to it. Every edit locks the nights it touches through
// package inventory and is written to the audit log.
package admin

import (
	"context"
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
	"kaapi/api/internal/pricing"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Grid returns every room type's nights in the range, priced on one plan.
func (s *Service) Grid(ctx context.Context, stay domain.Stay, planCode string) (Grid, error) {
	if n := stay.Nights(); n < 1 || n > maxGridDays {
		return Grid{}, apperr.Validation(map[string]string{"to": fmt.Sprintf("must be 1 to %d nights after from", maxGridDays)})
	}
	nights, err := s.store.grid(ctx, s.pool, stay, planCode)
	if err != nil {
		return Grid{}, err
	}
	if len(nights) == 0 {
		return Grid{}, ErrPlanNotFound
	}
	g := Grid{Plan: planCode, Stay: stay}
	for _, gn := range nights {
		n := gn.night
		n.Left = max(n.Allotment-n.Sold-n.Held, 0)
		if len(g.Rows) == 0 || g.Rows[len(g.Rows)-1].RoomTypeID != gn.roomTypeID {
			g.Rows = append(g.Rows, Row{RoomTypeID: gn.roomTypeID, Slug: gn.slug, Name: gn.name, TotalKeys: gn.totalKeys})
		}
		r := &g.Rows[len(g.Rows)-1]
		r.Nights = append(r.Nights, n)
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
		keys, err := s.store.totalKeys(ctx, tx, c.RoomTypeID)
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
			if err := s.store.updateInventory(ctx, tx, c, dates, actor); err != nil {
				return err
			}
		}
		if c.RoomOnlyPricePaise != nil {
			if err := s.reprice(ctx, tx, c.RoomTypeID, dates, *c.RoomOnlyPricePaise, actor); err != nil {
				return err
			}
		}
		return s.store.audit(ctx, tx, actor, c.RoomTypeID, auditChange(c, len(dates)))
	})
	if err != nil {
		return 0, err
	}
	return len(dates), nil
}

// reprice sets the room-only price and derives every other plan from it.
func (s *Service) reprice(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, dates []time.Time, roomOnly int64, actor uuid.UUID) error {
	plans, err := s.store.plans(ctx, tx, roomTypeID)
	if err != nil {
		return err
	}
	for _, p := range plans {
		price := roomOnly
		if p.code != RoomOnlyPlan {
			price = pricing.DerivePlanPrice(roomOnly, p.add, p.bps, planRoundingPaise)
		}
		if err := s.store.setPrice(ctx, tx, p.id, dates, price, actor); err != nil {
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
