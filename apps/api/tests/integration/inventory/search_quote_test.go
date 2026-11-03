package inventory_test

import (
	"testing"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/tests/testdb"
)

// The search prices every plan from data it loads in one batch; a hold prices one plan on
// its own. Both must give the same quote, or a guest would see one price and be held at
// another. Checked with a typed code and an automatic offer both in play.
func TestSearchQuotesMatchWhatAHoldFreezes(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	if _, err := pool.Exec(ctx, `INSERT INTO promo_codes (id, code, description, type, value, stay_from, stay_to, book_from, book_to, auto_apply)
		VALUES ($1, 'SEASON12', 'Season: 12% off', 'PERCENT', 1200, '2026-11-01', '2026-11-30', '2026-10-01', '2026-11-30', true)`, uuid.New()); err != nil {
		t.Fatal(err)
	}
	svc := inventory.NewService(pool, today)
	for _, code := range []string{"", "DIRECT10", "NOSUCHCODE"} {
		stay := domain.Stay{CheckIn: day("2026-11-21"), CheckOut: day("2026-11-23")}
		party := domain.Party{Adults: 2, Rooms: 1}
		opts, err := svc.Search(ctx, inventory.SearchRequest{Stay: stay, Party: party, PromoCode: code})
		if err != nil {
			t.Fatal(err)
		}
		checked := 0
		for _, o := range opts {
			for _, p := range o.Plans {
				// An unknown code falls back to the best price without it, so the hold does too.
				held := code
				if o.PromoError != "" {
					held = ""
				}
				h, err := svc.CreateHold(ctx, inventory.HoldRequest{RoomTypeID: o.RoomTypeID, RatePlanID: p.PlanID, Stay: stay, Party: party, PromoCode: held})
				if err != nil {
					t.Fatalf("%s %s code %q: hold: %v", o.Slug, p.Code, code, err)
				}
				if h.Quote.TotalPaise != p.Quote.TotalPaise || h.Quote.DiscountPaise != p.Quote.DiscountPaise ||
					h.Quote.TaxPaise != p.Quote.TaxPaise || h.Quote.PromoCode != p.Quote.PromoCode {
					t.Errorf("%s %s code %q: search %+v, hold %+v", o.Slug, p.Code, code, p.Quote, h.Quote)
				}
				if err := svc.Release(ctx, h.ID); err != nil {
					t.Fatal(err)
				}
				checked++
			}
		}
		if checked < 6 {
			t.Fatalf("code %q: only %d plans priced; the comparison proves little", code, checked)
		}
	}
}
