package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/apperr"
	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/pricing"
)

// Querier is satisfied by both a pool and a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type settings struct {
	location    *time.Location
	holdMinutes int
	maxNights   int
	maxRooms    int
	slabs       []pricing.Slab
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
		st.location = time.FixedZone("IST", 5*3600+1800) // tzdata missing on a slim container
	}
	return st, json.Unmarshal(slabs, &st.slabs)
}

// quoteStay prices a stay from the stored nightly rates.
func quoteStay(ctx context.Context, q Querier, req HoldRequest, st settings, today time.Time) (pricing.Quote, *uuid.UUID, error) {
	var room pricing.Room
	var planMinNights int
	err := q.QueryRow(ctx, `SELECT rt.base_occupancy, rt.max_adults, rt.max_children, rt.extra_adult_paise, rp.min_nights
		FROM rate_plans rp JOIN room_types rt ON rt.id = rp.room_type_id
		WHERE rp.id = $1 AND rt.id = $2 AND rp.is_active AND rt.is_active`, req.RatePlanID, req.RoomTypeID).
		Scan(&room.BaseOccupancy, &room.MaxAdults, &room.MaxChildren, &room.ExtraAdultPaise, &planMinNights)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.Quote{}, nil, ErrPlanMismatch
	}
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	if n := nightsBetween(req.CheckIn, req.CheckOut); n < planMinNights {
		return pricing.Quote{}, nil, &RestrictionError{Err: ErrMinStay, Date: req.CheckIn, MinStay: planMinNights, Reason: "rate plan minimum"}
	}

	rows, err := q.Query(ctx, `SELECT date, price_paise FROM night_rates
		WHERE rate_plan_id = $1 AND date >= $2 AND date < $3 ORDER BY date`, req.RatePlanID, req.CheckIn, req.CheckOut)
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	nights, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (pricing.Night, error) {
		var n pricing.Night
		return n, r.Scan(&n.Date, &n.PricePaise)
	})
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	if len(nights) != nightsBetween(req.CheckIn, req.CheckOut) {
		return pricing.Quote{}, nil, ErrNotForSale
	}

	var promo *pricing.Promo
	var promoID *uuid.UUID
	if code := strings.ToUpper(strings.TrimSpace(req.PromoCode)); code != "" {
		rule, id, err := loadPromo(ctx, q, code)
		if err != nil {
			return pricing.Quote{}, nil, err
		}
		if err := pricing.Eligible(rule, req.CheckIn, req.CheckOut, today, req.RoomTypeID); err != nil {
			return pricing.Quote{}, nil, &PromoError{Err: err}
		}
		promo, promoID = &rule.Promo, &id
	}

	q2, err := pricing.Price(pricing.Input{Nights: nights, Rooms: req.Rooms, Adults: req.Adults, Children: req.Children,
		Room: room, Promo: promo, Slabs: st.slabs})
	if errors.Is(err, pricing.ErrTooManyGuest) {
		return q2, nil, apperr.Wrap(err, apperr.CodeTooManyGuests, ErrTooManyGuests.Message)
	}
	return q2, promoID, err
}

func loadPromo(ctx context.Context, q Querier, code string) (pricing.PromoRule, uuid.UUID, error) {
	var r pricing.PromoRule
	var id uuid.UUID
	var typ string
	err := q.QueryRow(ctx, `SELECT id, code, type, value, stay_from, stay_to, book_from, book_to, min_nights, max_uses, used_count,
		coalesce(room_type_ids, '{}'), is_active FROM promo_codes WHERE code = $1`, code).
		Scan(&id, &r.Code, &typ, &r.Value, &r.StayFrom, &r.StayTo, &r.BookFrom, &r.BookTo, &r.MinNights, &r.MaxUses, &r.UsedCount,
			&r.RoomTypeIDs, &r.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, id, &PromoError{Err: errPromoUnknown}
	}
	r.Type = pricing.PromoType(typ)
	return r, id, err
}
