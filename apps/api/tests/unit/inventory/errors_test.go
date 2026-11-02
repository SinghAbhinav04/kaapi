package inventory_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/pricing"

	"kaapi/api/internal/inventory"
)

func d(s string) time.Time { t, _ := time.Parse(time.DateOnly, s); return t }

// Every inventory error must reach clients with a stable code, the right status and
// enough detail for the UI to explain itself.
func TestDomainErrorsMapToResponses(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   apperr.Code
		status int
		detail string
		want   any
	}{
		{"sold out names nights", fmt.Errorf("hold: %w", &inventory.SoldOutError{Nights: []time.Time{d("2026-12-30"), d("2026-12-31")}}),
			apperr.CodeSoldOut, http.StatusConflict, "nights", []string{"2026-12-30", "2026-12-31"}},
		{"minimum stay", &inventory.RestrictionError{Err: inventory.ErrMinStay, Date: d("2026-11-06"), MinStay: 2},
			apperr.CodeStayRestricted, http.StatusUnprocessableEntity, "rule", inventory.RuleMinStay},
		{"stop sell", &inventory.RestrictionError{Err: inventory.ErrStopSell, Date: d("2027-01-14")},
			apperr.CodeStayRestricted, http.StatusUnprocessableEntity, "rule", inventory.RuleStopSell},
		{"expired promo", &inventory.PromoError{Err: pricing.ErrPromoExpired},
			apperr.CodePromoInvalid, http.StatusUnprocessableEntity, "reason", inventory.PromoExpired},
		{"allotment", &inventory.AllotmentError{Date: d("2026-11-21"), Sold: 0, Held: 1, RequestedTo: 0},
			apperr.CodeAllotmentTooLow, http.StatusConflict, "held", 1},
		{"hold ended", inventory.ErrHoldNotActive, apperr.CodeHoldExpired, http.StatusGone, "", nil},
		{"invalid stay", inventory.ErrInvalidStay,
			apperr.CodeInvalidStay, http.StatusUnprocessableEntity, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ae := apperr.From(tc.err)
			if ae.Code != tc.code || ae.Status() != tc.status {
				t.Fatalf("got %s/%d, want %s/%d", ae.Code, ae.Status(), tc.code, tc.status)
			}
			if ae.Message == "" || ae.Message == "Something went wrong on our side." {
				t.Fatalf("needs a specific message, got %q", ae.Message)
			}
			if tc.detail != "" && fmt.Sprint(ae.Details[tc.detail]) != fmt.Sprint(tc.want) {
				t.Fatalf("details[%s] = %v, want %v", tc.detail, ae.Details[tc.detail], tc.want)
			}
		})
	}
}

func TestSentinelsStillMatchWithErrorsIs(t *testing.T) {
	if !errors.Is(&inventory.SoldOutError{}, inventory.ErrSoldOut) || !errors.Is(apperr.New(apperr.CodeInvalidStay, "Check-in can't be in the past."), inventory.ErrInvalidStay) ||
		!errors.Is(&inventory.RestrictionError{Err: inventory.ErrClosedToArrival}, inventory.ErrClosedToArrival) ||
		!errors.Is(&inventory.AllotmentError{}, inventory.ErrAllotmentTooLow) {
		t.Fatal("errors.Is must keep working for callers")
	}
	if errors.Is(&inventory.RestrictionError{Err: inventory.ErrMinStay}, inventory.ErrStopSell) {
		t.Fatal("different restrictions must not match each other")
	}
}

func TestRestrictionMessageIsHuman(t *testing.T) {
	ae := (&inventory.RestrictionError{Err: inventory.ErrMinStay, Date: d("2026-11-06"), MinStay: 2}).AppError()
	if ae.Message != "Stays arriving on Fri 6 Nov need at least 2 nights." {
		t.Fatalf("%q", ae.Message)
	}
}
