package inventory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/pricing"
)

// istFallback is used if the container has no time zone database.
var istFallback = time.FixedZone("IST", 5*3600+1800)

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
	plan, err := store{}.planFor(ctx, q, req.RoomTypeID, req.RatePlanID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.Quote{}, nil, ErrPlanMismatch
	}
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	nights, err := store{}.nightRates(ctx, q, req.RatePlanID, req.CheckIn, req.CheckOut)
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
