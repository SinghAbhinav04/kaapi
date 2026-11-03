package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// diwali covers the 14–16 Nov villa stay, bookable on the seed's "today" (2 Nov), at 15% off:
// better than the seeded DIRECT10 code's 10%.
func diwali() api.OfferRequest {
	return api.OfferRequest{Code: "DIWALI26", Description: "Diwali on the estate: 15% off", Type: domain.PromoPercent, Value: 1500,
		StayFrom: "2026-11-01", StayTo: "2026-11-30", BookFrom: "2026-10-20", BookTo: "2026-11-20", MinNights: 1,
		RoomTypeIDs: []uuid.UUID{}, AutoApply: true, Active: true}
}

// villaQuote is the Coffee Villa's breakfast-plan quote for 14–16 Nov, with an optional typed code.
func villaQuote(t *testing.T, s *apitest.Server, promo string) api.QuoteDTO {
	t.Helper()
	var av api.AvailabilityResponse
	s.Do(http.MethodGet, "/v1/availability?check_in=2026-11-14&check_out=2026-11-16&adults=2&children=1&promo="+promo, nil, nil).JSON(t, &av)
	for _, o := range av.Options {
		if o.Slug == "coffee-villa" {
			for _, p := range o.Plans {
				if p.Code == "BB-FLEX" {
					return p.Quote
				}
			}
		}
	}
	t.Fatal("no Coffee Villa breakfast quote")
	return api.QuoteDTO{}
}

func TestFestiveOfferAppliesByItselfAndGuestsGetTheBestDeal(t *testing.T) {
	s := apitest.New(t)
	before := villaQuote(t, s, "")
	if before.DiscountPaise != 0 {
		t.Fatalf("no offer yet: %+v", before)
	}
	typed := villaQuote(t, s, "DIRECT10")

	staffLogin(t, s, "gm@kaapiestate.test", nil)
	var created api.OfferResponse
	res := s.Do(http.MethodPost, "/v1/staff/offers", diwali(), nil)
	res.JSON(t, &created)
	if res.Status != http.StatusCreated || created.Offer.UsedCount != 0 || !created.Offer.AutoApply {
		t.Fatalf("create: %d %+v", res.Status, created.Offer)
	}

	// No code typed: the festive offer applies by itself, reducing the rate-table price.
	auto := villaQuote(t, s, "")
	if auto.PromoCode == nil || *auto.PromoCode != "DIWALI26" || auto.DiscountPaise != before.SubtotalPaise*15/100 {
		t.Fatalf("automatic offer: %+v", auto)
	}
	// A weaker typed code doesn't take the guest's better deal away.
	if q := villaQuote(t, s, "DIRECT10"); q.PromoCode == nil || *q.PromoCode != "DIWALI26" || q.TotalPaise != auto.TotalPaise {
		t.Fatalf("best of typed and automatic: %+v", q)
	}

	// Paused: back to the typed code alone, and nothing without one.
	pause := diwali()
	pause.Active = false
	var updated api.OfferResponse
	s.Do(http.MethodPut, "/v1/staff/offers/"+created.Offer.ID.String(), pause, nil).JSON(t, &updated)
	if updated.Offer.Active {
		t.Fatal("pause didn't stick")
	}
	if q := villaQuote(t, s, ""); q.DiscountPaise != 0 {
		t.Fatalf("a paused offer still applied: %+v", q)
	}
	if q := villaQuote(t, s, "DIRECT10"); q.TotalPaise != typed.TotalPaise {
		t.Fatalf("typed code after pause: %+v", q)
	}

	var audits int
	if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) FROM audit_logs WHERE entity = 'promo_code' AND entity_id = $1`,
		created.Offer.ID.String()).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("every change is audit-logged: %d %v", audits, err)
	}
}

func TestOfferAdminChecksPermissionAndInput(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "reservations@kaapiestate.test", nil)
	if res := s.Do(http.MethodGet, "/v1/staff/offers", nil, nil); res.Status != http.StatusForbidden {
		t.Fatalf("reservations can't manage offers: %d", res.Status)
	}

	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	tooBig := diwali()
	tooBig.Value = 6000 // 60%
	tooBig.StayTo = "2026-10-01"
	e := gm.Do(http.MethodPost, "/v1/staff/offers", tooBig, nil).Error(t)
	fields, _ := e.Details["fields"].(map[string]any)
	if e.Code != apperr.CodeValidation || fields["value"] == nil || fields["stay_to"] == nil {
		t.Fatalf("validation: %+v", e)
	}
	dup := diwali()
	dup.Code = "direct10" // the seeded code, typed in lower case
	if e := gm.Do(http.MethodPost, "/v1/staff/offers", dup, nil).Error(t); e.Code != apperr.CodeConflict {
		t.Fatalf("duplicate code: %s", e.Code)
	}

	var list api.OffersResponse
	gm.Do(http.MethodGet, "/v1/staff/offers", nil, nil).JSON(t, &list)
	if len(list.Offers) == 0 {
		t.Fatal("the seeded promo codes should be listed")
	}
}
