package inventory

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/apperr"
	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/pricing"
)

// Sentinels, matched with errors.Is. Their messages are what guests and staff see.
var (
	ErrInvalidStay     = apperr.New(apperr.CodeInvalidStay, "Those dates don't work.")
	ErrNotForSale      = apperr.New(apperr.CodeNotOnSale, "These dates aren't on sale yet.")
	ErrSoldOut         = apperr.New(apperr.CodeSoldOut, "There's nothing left for these dates.")
	ErrHoldNotFound    = apperr.New(apperr.CodeHoldNotFound, "We couldn't find that reservation hold.")
	ErrHoldNotActive   = apperr.New(apperr.CodeHoldExpired, "This hold has ended. Please check availability again.")
	ErrAllotmentTooLow = apperr.New(apperr.CodeAllotmentTooLow, "The allotment can't go below rooms already sold or on hold.")
	ErrPlanMismatch    = apperr.New(apperr.CodeValidation, "That rate isn't offered for this room.")
	ErrTooManyGuests   = apperr.New(apperr.CodeTooManyGuests, "Too many guests for the rooms chosen.")
)

func invalidStay(msg string) *apperr.Error { return apperr.New(apperr.CodeInvalidStay, msg) }

// SoldOutError lists the nights that have no room left, so the guest sees which date is the problem.
type SoldOutError struct{ Nights []time.Time }

func (e *SoldOutError) Error() string {
	return "inventory: sold out on " + strings.Join(dates(e.Nights), ", ")
}

func (e *SoldOutError) Is(target error) bool { return target == ErrSoldOut }

func (e *SoldOutError) AppError() *apperr.Error {
	return ErrSoldOut.With(apperr.Details{"nights": dates(e.Nights)})
}

// Rule names the stay restriction a request broke.
type Rule string

const (
	RuleStopSell          Rule = "STOP_SELL"
	RuleMinStay           Rule = "MIN_STAY"
	RuleClosedToArrival   Rule = "CLOSED_TO_ARRIVAL"
	RuleClosedToDeparture Rule = "CLOSED_TO_DEPARTURE"
)

// Restriction sentinels, for errors.Is in callers and tests.
var (
	ErrStopSell          = errors.New("inventory: closed for sale")
	ErrMinStay           = errors.New("inventory: minimum stay not met")
	ErrClosedToArrival   = errors.New("inventory: no arrivals on this date")
	ErrClosedToDeparture = errors.New("inventory: no departures on this date")
)

var ruleOf = map[error]Rule{
	ErrStopSell: RuleStopSell, ErrMinStay: RuleMinStay,
	ErrClosedToArrival: RuleClosedToArrival, ErrClosedToDeparture: RuleClosedToDeparture,
}

// RestrictionError explains a stay rule the guest's dates break.
type RestrictionError struct {
	Err     error
	Date    time.Time
	MinStay int
	Reason  string
}

func (e *RestrictionError) Error() string {
	return fmt.Sprintf("%v (%s)", e.Err, e.Date.Format(time.DateOnly))
}

func (e *RestrictionError) Unwrap() error { return e.Err }

func (e *RestrictionError) AppError() *apperr.Error {
	day := e.Date.Format("Mon 2 Jan")
	var msg string
	switch e.Err {
	case ErrStopSell:
		msg = "We aren't taking bookings for " + day + "."
	case ErrMinStay:
		msg = fmt.Sprintf("Stays arriving on %s need at least %d nights.", day, e.MinStay)
	case ErrClosedToArrival:
		msg = "We can't take arrivals on " + day + "."
	case ErrClosedToDeparture:
		msg = "We can't take departures on " + day + "."
	}
	d := apperr.Details{"rule": ruleOf[e.Err], "date": e.Date.Format(time.DateOnly)}
	if e.MinStay > 0 {
		d["min_stay"] = e.MinStay
	}
	return apperr.Wrap(e, apperr.CodeStayRestricted, msg).With(d)
}

// AllotmentError says why an allotment change was refused.
type AllotmentError struct {
	Date        time.Time
	Sold, Held  int
	RequestedTo int
}

func (e *AllotmentError) Error() string {
	return fmt.Sprintf("inventory: %s has %d sold and %d on hold, so the allotment can't go to %d",
		e.Date.Format(time.DateOnly), e.Sold, e.Held, e.RequestedTo)
}

func (e *AllotmentError) Is(target error) bool { return target == ErrAllotmentTooLow }

func (e *AllotmentError) AppError() *apperr.Error {
	return apperr.Wrap(e, apperr.CodeAllotmentTooLow,
		fmt.Sprintf("%s already has %d sold and %d on hold, so keep at least %d rooms online.",
			e.Date.Format("Mon 2 Jan"), e.Sold, e.Held, e.Sold+e.Held)).
		With(apperr.Details{"date": e.Date.Format(time.DateOnly), "sold": e.Sold, "held": e.Held})
}

// PromoReason says why a code didn't apply.
type PromoReason string

const (
	PromoUnknown   PromoReason = "UNKNOWN"
	PromoInactive  PromoReason = "INACTIVE"
	PromoNotYet    PromoReason = "NOT_YET_VALID"
	PromoExpired   PromoReason = "EXPIRED"
	PromoStayDates PromoReason = "STAY_DATES"
	PromoMinNights PromoReason = "MIN_NIGHTS"
	PromoRoomType  PromoReason = "ROOM_TYPE"
	PromoUsedUp    PromoReason = "USED_UP"
)

var errPromoUnknown = errors.New("we don't recognise this code")

var promoReasonOf = map[error]PromoReason{
	errPromoUnknown:           PromoUnknown,
	pricing.ErrPromoInactive:  PromoInactive,
	pricing.ErrPromoNotYet:    PromoNotYet,
	pricing.ErrPromoExpired:   PromoExpired,
	pricing.ErrPromoStayDates: PromoStayDates,
	pricing.ErrPromoMinNights: PromoMinNights,
	pricing.ErrPromoRoomType:  PromoRoomType,
	pricing.ErrPromoUsedUp:    PromoUsedUp,
}

// PromoError wraps a pricing eligibility error so the API can show its message.
type PromoError struct{ Err error }

func (e *PromoError) Error() string { return e.Err.Error() }
func (e *PromoError) Unwrap() error { return e.Err }

func (e *PromoError) AppError() *apperr.Error {
	msg := e.Err.Error()
	return apperr.Wrap(e, apperr.CodePromoInvalid, strings.ToUpper(msg[:1])+msg[1:]+".").
		With(apperr.Details{"reason": promoReasonOf[e.Err]})
}

func dates(ds []time.Time) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Format(time.DateOnly)
	}
	return out
}
