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

	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/pricing"
)

// istFallback is used if the container has no time zone database.
var istFallback = time.FixedZone("IST", 5*3600+1800)

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

// planInfo is what pricing one plan needs besides its nightly rates.
type planInfo struct {
	room      pricing.Room
	minNights int
}

// typedCode is the code the guest typed: its rule, or why it can't be used (unknown code).
type typedCode struct {
	rule pricing.PromoRule
	id   uuid.UUID
	err  error // a *PromoError when no such code exists
}

// quoteStay prices one plan for a stay (the hold path): it loads what that needs, then
// priceStay does the rest.
func quoteStay(ctx context.Context, q Querier, req HoldRequest, st settings, today time.Time) (pricing.Quote, *uuid.UUID, error) {
	var plan planInfo
	err := q.QueryRow(ctx, `SELECT rt.base_occupancy, rt.max_adults, rt.max_children, rt.extra_adult_paise, rp.min_nights
		FROM rate_plans rp JOIN room_types rt ON rt.id = rp.room_type_id
		WHERE rp.id = $1 AND rt.id = $2 AND rp.is_active AND rt.is_active`, req.RatePlanID, req.RoomTypeID).
		Scan(&plan.room.BaseOccupancy, &plan.room.MaxAdults, &plan.room.MaxChildren, &plan.room.ExtraAdultPaise, &plan.minNights)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.Quote{}, nil, ErrPlanMismatch
	}
	if err != nil {
		return pricing.Quote{}, nil, err
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
	typed, err := loadTyped(ctx, q, req.PromoCode)
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	autos, err := loadAutoOffers(ctx, q, today)
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	return priceStay(nights, plan, req, st, today, typed, autos)
}

// loadTyped looks up the code the guest typed; nil when they typed none.
func loadTyped(ctx context.Context, q Querier, code string) (*typedCode, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, nil
	}
	rule, id, err := loadPromo(ctx, q, code)
	var pe *PromoError
	if errors.As(err, &pe) {
		return &typedCode{err: pe}, nil
	}
	if err != nil {
		return nil, err
	}
	return &typedCode{rule: rule, id: id}, nil
}

// priceStay checks the plan's restrictions, picks the guest's best offer (the code they
// typed, or any automatic offer that fits; an invalid code is reported) and prices the stay.
// It does no I/O: the hold and the search both call it with what they loaded, so the price
// a guest sees in search is exactly the price their hold freezes.
func priceStay(nights []pricing.Night, plan planInfo, req HoldRequest, st settings, today time.Time, typed *typedCode,
	autos []autoOffer) (pricing.Quote, *uuid.UUID, error) {
	if req.Nights() < plan.minNights {
		return pricing.Quote{}, nil, &RestrictionError{Err: ErrMinStay, Date: req.CheckIn, MinStay: plan.minNights, Reason: "rate plan minimum"}
	}
	if len(nights) != req.Nights() {
		return pricing.Quote{}, nil, ErrNotForSale
	}

	var offers []candidate
	if typed != nil {
		if typed.err != nil {
			return pricing.Quote{}, nil, typed.err
		}
		if err := pricing.Eligible(typed.rule, req.Stay, today, req.RoomTypeID); err != nil {
			return pricing.Quote{}, nil, &PromoError{Err: err}
		}
		offers = append(offers, candidate{promo: typed.rule.Promo, id: typed.id})
	}
	for _, a := range autos {
		if pricing.Eligible(a.rule, req.Stay, today, req.RoomTypeID) == nil {
			offers = append(offers, candidate{promo: a.rule.Promo, id: a.id})
		}
	}

	in := pricing.Input{Nights: nights, Party: req.Party, Room: plan.room, Slabs: st.slabs}
	best, err := pricing.Price(in)
	if errors.Is(err, pricing.ErrTooManyGuest) {
		return best, nil, apperr.Wrap(err, apperr.CodeTooManyGuests, ErrTooManyGuests.Message)
	}
	if err != nil {
		return best, nil, err
	}
	var bestID *uuid.UUID
	for _, o := range offers {
		in.Promo = &o.promo
		quote, err := pricing.Price(in)
		if err != nil {
			return quote, nil, err
		}
		// Strictly cheaper wins, so on a tie the code the guest typed (listed first) stays.
		if bestID == nil || quote.TotalPaise < best.TotalPaise {
			best, bestID = quote, &o.id
		}
	}
	return best, bestID, nil
}

// candidate is one discount that could apply to a stay.
type candidate struct {
	promo pricing.Promo
	id    uuid.UUID
}

type autoOffer struct {
	rule pricing.PromoRule
	id   uuid.UUID
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
