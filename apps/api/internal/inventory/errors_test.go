package inventory

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/apperr"
	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/pricing"
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
		{"sold out names nights", fmt.Errorf("hold: %w", &SoldOutError{Nights: []time.Time{d("2026-12-30"), d("2026-12-31")}}),
			apperr.CodeSoldOut, http.StatusConflict, "nights", []string{"2026-12-30", "2026-12-31"}},
		{"minimum stay", &RestrictionError{Err: ErrMinStay, Date: d("2026-11-06"), MinStay: 2},
			apperr.CodeStayRestricted, http.StatusUnprocessableEntity, "rule", RuleMinStay},
		{"stop sell", &RestrictionError{Err: ErrStopSell, Date: d("2027-01-14")},
			apperr.CodeStayRestricted, http.StatusUnprocessableEntity, "rule", RuleStopSell},
		{"expired promo", &PromoError{Err: pricing.ErrPromoExpired},
			apperr.CodePromoInvalid, http.StatusUnprocessableEntity, "reason", PromoExpired},
		{"allotment", &AllotmentError{Date: d("2026-11-21"), Sold: 0, Held: 1, RequestedTo: 0},
			apperr.CodeAllotmentTooLow, http.StatusConflict, "held", 1},
		{"hold ended", ErrHoldNotActive, apperr.CodeHoldExpired, http.StatusGone, "", nil},
		{"invalid stay keeps its specific message", invalidStay("Check-in can't be in the past."),
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
	if !errors.Is(&SoldOutError{}, ErrSoldOut) || !errors.Is(invalidStay("x"), ErrInvalidStay) ||
		!errors.Is(&RestrictionError{Err: ErrClosedToArrival}, ErrClosedToArrival) ||
		!errors.Is(&AllotmentError{}, ErrAllotmentTooLow) {
		t.Fatal("errors.Is must keep working for callers")
	}
	if errors.Is(&RestrictionError{Err: ErrMinStay}, ErrStopSell) {
		t.Fatal("different restrictions must not match each other")
	}
}

func TestRestrictionMessageIsHuman(t *testing.T) {
	ae := (&RestrictionError{Err: ErrMinStay, Date: d("2026-11-06"), MinStay: 2}).AppError()
	if ae.Message != "Stays arriving on Fri 6 Nov need at least 2 nights." {
		t.Fatalf("%q", ae.Message)
	}
}
