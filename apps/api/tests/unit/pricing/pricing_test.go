package pricing_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/pricing"
)

func ptr(v int64) *int64 { return &v }

// The resort's slabs: up to ₹7,500 a night is 5%, above that 18%.
var slabs = []pricing.Slab{{MaxTariffPaise: ptr(750_000), RateBps: 500}, {RateBps: 1800}}

var (
	plantation = pricing.Room{BaseOccupancy: 2, MaxAdults: 3, MaxChildren: 1, ExtraAdultPaise: 150_000}
	villa      = pricing.Room{BaseOccupancy: 2, MaxAdults: 4, MaxChildren: 2, ExtraAdultPaise: 200_000}
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func nights(start string, prices ...int64) []pricing.Night {
	out := make([]pricing.Night, len(prices))
	for i, p := range prices {
		out[i] = pricing.Night{Date: day(start).AddDate(0, 0, i), PricePaise: p}
	}
	return out
}

func party(rooms, adults, children int) domain.Party {
	return domain.Party{Rooms: rooms, Adults: adults, Children: children}
}

func percent(code string, bps int64) *pricing.Promo {
	return &pricing.Promo{Code: code, Type: domain.PromoPercent, Value: bps}
}

func TestPriceMatchesDesignBoards(t *testing.T) {
	tests := []struct {
		name      string
		in        pricing.Input
		wantTax   int64
		wantTotal int64
		wantRates []int64
	}{
		{
			// Room page (phone): Saturday's weekend rate crosses ₹7,500, Sunday's doesn't.
			name:      "plantation room only, 14-16 Nov, slab crossing",
			in:        pricing.Input{Nights: nights("2026-11-14", 780_000, 680_000), Party: party(1, 2, 1), Room: plantation, Slabs: slabs},
			wantTax:   140_400 + 34_000,
			wantTotal: 1_634_400, // ₹16,344
			wantRates: []int64{1800, 500},
		},
		{
			name:      "coffee villa breakfast, 14-16 Nov",
			in:        pricing.Input{Nights: nights("2026-11-14", 2_130_000, 1_870_000), Party: party(1, 2, 1), Room: villa, Slabs: slabs},
			wantTax:   720_000,
			wantTotal: 4_720_000, // ₹47,200
			wantRates: []int64{1800, 1800},
		},
		{
			// Checkout board: DIRECT10 on the same stay.
			name:      "coffee villa breakfast with DIRECT10",
			in:        pricing.Input{Nights: nights("2026-11-14", 2_130_000, 1_870_000), Party: party(1, 2, 1), Room: villa, Slabs: slabs, Promo: percent("DIRECT10", 1000)},
			wantTax:   648_000,
			wantTotal: 4_248_000, // ₹42,480
			wantRates: []int64{1800, 1800},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q, err := pricing.Price(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if q.TaxPaise != tc.wantTax || q.TotalPaise != tc.wantTotal {
				t.Fatalf("tax %d total %d, want tax %d total %d", q.TaxPaise, q.TotalPaise, tc.wantTax, tc.wantTotal)
			}
			for i, want := range tc.wantRates {
				if q.Lines[i].GSTRateBps != want {
					t.Errorf("night %d GST rate %d, want %d", i, q.Lines[i].GSTRateBps, want)
				}
			}
			if q.TotalPaise != q.SubtotalPaise-q.DiscountPaise+q.TaxPaise {
				t.Error("total doesn't add up (the database would reject this booking)")
			}
		})
	}
}

func TestSlabBoundary(t *testing.T) {
	for _, tc := range []struct {
		price int64
		want  int64
	}{
		{750_000, 500},  // exactly ₹7,500: lower band
		{750_100, 1800}, // ₹7,501: upper band
		{100_000, 500},
	} {
		q, err := pricing.Price(pricing.Input{Nights: nights("2026-11-10", tc.price), Party: party(1, 2, 0), Room: plantation, Slabs: slabs})
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Lines[0].GSTRateBps; got != tc.want {
			t.Errorf("price %d: rate %d, want %d", tc.price, got, tc.want)
		}
	}
}

func TestPromoCanMoveNightIntoLowerSlab(t *testing.T) {
	// ₹8,000 less 10% is ₹7,200, so GST is charged at 5% on the amount actually paid.
	q, err := pricing.Price(pricing.Input{Nights: nights("2026-11-10", 800_000), Party: party(1, 2, 0), Room: plantation, Slabs: slabs,
		Promo: percent("DIRECT10", 1000)})
	if err != nil {
		t.Fatal(err)
	}
	if l := q.Lines[0]; l.TaxablePaise != 720_000 || l.GSTRateBps != 500 || l.GSTPaise != 36_000 {
		t.Fatalf("got taxable %d rate %d gst %d", l.TaxablePaise, l.GSTRateBps, l.GSTPaise)
	}
}

func TestSlabUsesPricePerRoomNotPerBooking(t *testing.T) {
	// Two rooms at ₹6,800 is ₹13,600 for the night, but each room is still in the 5% band.
	q, err := pricing.Price(pricing.Input{Nights: nights("2026-11-10", 680_000), Party: party(2, 4, 0), Room: plantation, Slabs: slabs})
	if err != nil {
		t.Fatal(err)
	}
	if q.Lines[0].GSTRateBps != 500 {
		t.Fatalf("rate %d, want 500", q.Lines[0].GSTRateBps)
	}
}

func TestExtraAdultIsCharged(t *testing.T) {
	q, err := pricing.Price(pricing.Input{Nights: nights("2026-11-10", 680_000), Party: party(1, 3, 0), Room: plantation, Slabs: slabs})
	if err != nil {
		t.Fatal(err)
	}
	if q.SubtotalPaise != 830_000 {
		t.Fatalf("subtotal %d, want 830000 (₹6,800 + ₹1,500 extra adult)", q.SubtotalPaise)
	}
	if q.Lines[0].GSTRateBps != 1800 {
		t.Fatal("the extra-adult charge is part of the tariff, so ₹8,300 is in the 18% band")
	}
}

func TestFlatPromoIsSpreadAndExact(t *testing.T) {
	// STAYLONGER: ₹3,000 off a 4-night villa stay, spread by night price.
	q, err := pricing.Price(pricing.Input{Nights: nights("2027-04-10", 2_010_000, 2_010_000, 1_750_000, 1_750_000), Party: party(1, 2, 0), Room: villa, Slabs: slabs,
		Promo: &pricing.Promo{Code: "STAYLONGER", Type: domain.PromoFlat, Value: 300_000}})
	if err != nil {
		t.Fatal(err)
	}
	if q.DiscountPaise != 300_000 {
		t.Fatalf("discount %d, want exactly 300000", q.DiscountPaise)
	}
	if q.Lines[0].DiscountPaise <= q.Lines[3].DiscountPaise {
		t.Error("the pricier night should carry more of the discount")
	}
}

func TestFlatPromoNeverExceedsSubtotal(t *testing.T) {
	q, err := pricing.Price(pricing.Input{Nights: nights("2026-11-10", 100_000), Party: party(1, 1, 0), Room: plantation, Slabs: slabs,
		Promo: &pricing.Promo{Code: "BIG", Type: domain.PromoFlat, Value: 900_000}})
	if err != nil {
		t.Fatal(err)
	}
	if q.TotalPaise != 0 || q.DiscountPaise != 100_000 {
		t.Fatalf("total %d discount %d", q.TotalPaise, q.DiscountPaise)
	}
}

func TestRejectsBadInput(t *testing.T) {
	base := pricing.Input{Nights: nights("2026-11-10", 680_000), Party: party(1, 2, 0), Room: plantation, Slabs: slabs}
	cases := map[string]func(*pricing.Input){
		"no nights":       func(in *pricing.Input) { in.Nights = nil },
		"no rooms":        func(in *pricing.Input) { in.Rooms = 0 },
		"too many adults": func(in *pricing.Input) { in.Adults = 4 },
		"too many kids":   func(in *pricing.Input) { in.Children = 2 },
		"open slab":       func(in *pricing.Input) { in.Slabs = slabs[:1] },
		"missing price":   func(in *pricing.Input) { in.Nights[0].PricePaise = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := base
			in.Nights = append([]pricing.Night(nil), base.Nights...)
			mutate(&in)
			if _, err := pricing.Price(in); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestDerivePlanPrice(t *testing.T) {
	// Coffee Villa Saturday: room only ₹20,100.
	if got := pricing.DerivePlanPrice(2_010_000, 120_000, 0, 10_000); got != 2_130_000 {
		t.Errorf("breakfast: %d", got)
	}
	// Non-refundable: (20,100 + 1,200) less 10% = 19,170, rounded to ₹19,200.
	if got := pricing.DerivePlanPrice(2_010_000, 120_000, 1000, 10_000); got != 1_920_000 {
		t.Errorf("non-refundable: %d", got)
	}
	// Weekend uplift rounding: 6,800 × 1.15 = 7,820 → ₹7,800.
	if got := pricing.RoundTo(782_000, 10_000); got != 780_000 {
		t.Errorf("round: %d", got)
	}
}

func TestEligible(t *testing.T) {
	villaID, roomID := uuid.New(), uuid.New()
	maxUses := 25
	r := pricing.PromoRule{
		Promo:    pricing.Promo{Code: "STAYLONGER", Type: domain.PromoFlat, Value: 300_000},
		StayFrom: day("2026-11-01"), StayTo: day("2027-04-30"),
		BookFrom: day("2026-10-15"), BookTo: day("2027-04-30"),
		MinNights: 4, MaxUses: &maxUses, RoomTypeIDs: []uuid.UUID{villaID}, Active: true,
	}
	today := day("2026-11-02")
	cases := []struct {
		name     string
		mutate   func(*pricing.PromoRule)
		in, out  string
		room     uuid.UUID
		bookedOn time.Time
		want     error
	}{
		{"ok", nil, "2026-11-20", "2026-11-24", villaID, today, nil},
		{"too short", nil, "2026-11-20", "2026-11-22", villaID, today, pricing.ErrPromoMinNights},
		{"wrong room", nil, "2026-11-20", "2026-11-24", roomID, today, pricing.ErrPromoRoomType},
		{"stay after window", nil, "2027-04-28", "2027-05-02", villaID, today, pricing.ErrPromoStayDates},
		{"last night inside window is fine", nil, "2027-04-27", "2027-05-01", villaID, today, nil},
		{"booked too early", nil, "2026-11-20", "2026-11-24", villaID, day("2026-10-01"), pricing.ErrPromoNotYet},
		{"expired", nil, "2026-11-20", "2026-11-24", villaID, day("2027-05-01"), pricing.ErrPromoExpired},
		{"used up", func(r *pricing.PromoRule) { r.UsedCount = 25 }, "2026-11-20", "2026-11-24", villaID, today, pricing.ErrPromoUsedUp},
		{"inactive", func(r *pricing.PromoRule) { r.Active = false }, "2026-11-20", "2026-11-24", villaID, today, pricing.ErrPromoInactive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := r
			if tc.mutate != nil {
				tc.mutate(&rule)
			}
			stay := domain.Stay{CheckIn: day(tc.in), CheckOut: day(tc.out)}
			if got := pricing.Eligible(rule, stay, tc.bookedOn, tc.room); !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
