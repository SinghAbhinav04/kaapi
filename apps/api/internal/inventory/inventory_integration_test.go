package inventory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/inventory"
	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/idgen"
	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/testdb"
)

// "Today" for every test is the date the seed data was written for.
var today = func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

type ids struct{ room, plan uuid.UUID }

func lookup(t *testing.T, ctx context.Context, pool *pgxpool.Pool, slug, plan string) ids {
	t.Helper()
	var x ids
	err := pool.QueryRow(ctx, `SELECT rt.id, rp.id FROM room_types rt JOIN rate_plans rp ON rp.room_type_id = rt.id
		WHERE rt.slug = $1 AND rp.code = $2`, slug, plan).Scan(&x.room, &x.plan)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func req(x ids, in, out string, rooms, adults, children int) inventory.HoldRequest {
	return inventory.HoldRequest{RoomTypeID: x.room, RatePlanID: x.plan, CheckIn: day(in), CheckOut: day(out), Rooms: rooms, Adults: adults, Children: children}
}

// The headline guarantee: 25 guests try to hold the last Pool Villa at the same moment.
// Exactly one gets it; the other 24 are told it's sold out. Nobody is oversold.
func TestLastRoomRace(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	pv := lookup(t, ctx, pool, "pool-villa", "RO-FLEX")

	const guests = 25
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, guests)
	for i := 0; i < guests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.CreateHold(ctx, req(pv, "2027-03-02", "2027-03-04", 1, 2, 0))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	won, soldOut := 0, 0
	for err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, inventory.ErrSoldOut):
			soldOut++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if won != 1 || soldOut != guests-1 {
		t.Fatalf("won %d, sold out %d; want exactly 1 and %d", won, soldOut, guests-1)
	}
	assertNeverOversold(t, ctx, pool)
}

// Many overlapping multi-night stays on a room type that is already half sold.
func TestOverlappingStaysNeverOversell(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	cv := lookup(t, ctx, pool, "coffee-villa", "RO-FLEX")

	starts := []string{"2026-11-12", "2026-11-13", "2026-11-14", "2026-11-15", "2026-11-16"}
	var wg sync.WaitGroup
	start := make(chan struct{})
	var mu sync.Mutex
	won := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := day(starts[i%len(starts)])
			<-start
			_, err := svc.CreateHold(ctx, inventory.HoldRequest{RoomTypeID: cv.room, RatePlanID: cv.plan,
				CheckIn: in, CheckOut: in.AddDate(0, 0, 2), Rooms: 1, Adults: 2})
			if err == nil {
				mu.Lock()
				won++
				mu.Unlock()
			} else if !errors.Is(err, inventory.ErrSoldOut) {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if won == 0 {
		t.Fatal("nobody got a villa, but some nights had space")
	}
	assertNeverOversold(t, ctx, pool)
}

func TestSoldOutNamesTheNights(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	// Pool Villa is booked 30 Dec – 2 Jan, so a 29 Dec – 1 Jan stay fails on the 30th and 31st.
	_, err := svc.CreateHold(ctx, req(lookup(t, ctx, pool, "pool-villa", "RO-FLEX"), "2026-12-29", "2027-01-01", 1, 2, 0))
	var so *inventory.SoldOutError
	if !errors.As(err, &so) || len(so.Nights) != 2 ||
		so.Nights[0].Format(time.DateOnly) != "2026-12-30" || so.Nights[1].Format(time.DateOnly) != "2026-12-31" {
		t.Fatalf("got %v", err)
	}
}

func TestStayRules(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	pr := lookup(t, ctx, pool, "plantation-room", "RO-FLEX")
	nrf := lookup(t, ctx, pool, "plantation-room", "BB-NRF")
	cases := []struct {
		name string
		r    inventory.HoldRequest
		want error
	}{
		{"wedding buyout is stop-sell", req(pr, "2027-01-14", "2027-01-15", 1, 2, 0), inventory.ErrStopSell},
		{"Diwali needs 2 nights", req(pr, "2026-11-06", "2026-11-07", 1, 2, 0), inventory.ErrMinStay},
		{"no arrivals on New Year's Eve", req(pr, "2026-12-31", "2027-01-02", 1, 2, 0), inventory.ErrClosedToArrival},
		{"non-refundable plan needs 2 nights", req(nrf, "2026-11-17", "2026-11-18", 1, 2, 0), inventory.ErrMinStay},
		{"past dates", req(pr, "2026-11-01", "2026-11-03", 1, 2, 0), inventory.ErrInvalidStay},
		{"check-out before check-in", req(pr, "2026-11-20", "2026-11-19", 1, 2, 0), inventory.ErrInvalidStay},
		{"beyond the rate calendar", req(pr, "2027-04-29", "2027-05-02", 1, 2, 0), inventory.ErrNotForSale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateHold(ctx, tc.r); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestExpiredHoldStopsCountingImmediately(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	pv := lookup(t, ctx, pool, "pool-villa", "RO-FLEX")

	first, err := svc.CreateHold(ctx, req(pv, "2027-03-02", "2027-03-04", 1, 2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateHold(ctx, req(pv, "2027-03-02", "2027-03-04", 1, 2, 0)); !errors.Is(err, inventory.ErrSoldOut) {
		t.Fatalf("second hold should be refused while the first is live, got %v", err)
	}
	expire(t, ctx, pool, first.ID)
	if _, err := svc.CreateHold(ctx, req(pv, "2027-03-02", "2027-03-04", 1, 2, 0)); err != nil {
		t.Fatalf("expired hold still blocks the room: %v", err)
	}
}

func TestConfirmSellsTheRoomOnce(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	cv := lookup(t, ctx, pool, "coffee-villa", "RO-FLEX")

	h, err := svc.CreateHold(ctx, req(cv, "2026-11-20", "2026-11-22", 1, 2, 0))
	if err != nil {
		t.Fatal(err)
	}
	bookingID := insertBooking(t, ctx, pool, cv, h)
	confirm := func() error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return inventory.Confirm(ctx, tx, h.ID, bookingID) })
	}
	if err := confirm(); err != nil {
		t.Fatal(err)
	}
	if got := sold(t, ctx, pool, cv.room, "2026-11-20"); got != 1 {
		t.Fatalf("sold %d on 20 Nov, want 1", got)
	}
	if err := confirm(); !errors.Is(err, inventory.ErrHoldNotActive) {
		t.Fatalf("second confirm (a repeated webhook) must not sell again, got %v", err)
	}
	if got := sold(t, ctx, pool, cv.room, "2026-11-20"); got != 1 {
		t.Fatalf("sold %d after repeat, want 1", got)
	}
	assertNeverOversold(t, ctx, pool)
}

func TestLatePayment(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	pv := lookup(t, ctx, pool, "pool-villa", "RO-FLEX")

	t.Run("room still free: booking goes through", func(t *testing.T) {
		h, err := svc.CreateHold(ctx, req(pv, "2027-03-02", "2027-03-04", 1, 2, 0))
		if err != nil {
			t.Fatal(err)
		}
		expire(t, ctx, pool, h.ID)
		b := insertBooking(t, ctx, pool, pv, h)
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return inventory.Confirm(ctx, tx, h.ID, b) }); err != nil {
			t.Fatalf("late payment with a free room should succeed: %v", err)
		}
	})

	t.Run("someone else took it: refund path", func(t *testing.T) {
		h, err := svc.CreateHold(ctx, req(pv, "2027-03-10", "2027-03-12", 1, 2, 0))
		if err != nil {
			t.Fatal(err)
		}
		expire(t, ctx, pool, h.ID)
		if _, err := svc.CreateHold(ctx, req(pv, "2027-03-10", "2027-03-12", 1, 2, 0)); err != nil {
			t.Fatal(err) // another guest takes the room after our hold lapsed
		}
		b := insertBooking(t, ctx, pool, pv, h)
		err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return inventory.Confirm(ctx, tx, h.ID, b) })
		if !errors.Is(err, inventory.ErrSoldOut) {
			t.Fatalf("got %v, want sold out so the payment is refunded", err)
		}
		assertNeverOversold(t, ctx, pool)
	})
}

func TestQuotesAndPromos(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	bb := lookup(t, ctx, pool, "coffee-villa", "BB-FLEX")

	r := req(bb, "2026-11-14", "2026-11-16", 1, 2, 1)
	r.PromoCode = "direct10" // case doesn't matter
	h, err := svc.CreateHold(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if h.Quote.TotalPaise != 4_248_000 {
		t.Fatalf("total %d, want ₹42,480 as on the checkout design", h.Quote.TotalPaise)
	}
	if err := svc.Release(ctx, h.ID); err != nil { // it held the last villa; give it back
		t.Fatal(err)
	}

	r.PromoCode = "MONSOON26"
	var pe *inventory.PromoError
	if _, err := svc.CreateHold(ctx, r); !errors.As(err, &pe) {
		t.Fatalf("expired code: got %v", err)
	}
	r.PromoCode = "NOPE"
	if _, err := svc.CreateHold(ctx, r); !errors.As(err, &pe) {
		t.Fatalf("unknown code: got %v", err)
	}
}

func TestHoldToken(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	h, err := svc.CreateHold(ctx, req(lookup(t, ctx, pool, "plantation-room", "RO-FLEX"), "2026-11-24", "2026-11-26", 1, 2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.VerifyToken(ctx, h.ID, h.Token); err != nil {
		t.Fatalf("own token rejected: %v", err)
	}
	if err := svc.VerifyToken(ctx, h.ID, h.Token+"x"); !errors.Is(err, inventory.ErrHoldNotFound) {
		t.Fatalf("wrong token accepted: %v", err)
	}
}

func TestSearchAndAlternatives(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)

	opts, err := svc.Search(ctx, inventory.SearchRequest{CheckIn: day("2026-11-14"), CheckOut: day("2026-11-16"), Rooms: 1, Adults: 2, Children: 1})
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]inventory.Option{}
	for _, o := range opts {
		bySlug[o.Slug] = o
	}
	cv := bySlug["coffee-villa"]
	if cv.Status != inventory.Available || cv.Left != 1 || len(cv.Plans) != 3 || cv.Plans[0].Quote.TotalPaise != 4_436_800 {
		t.Fatalf("coffee villa: %+v", cv)
	}
	if pr := bySlug["plantation-room"]; pr.Plans[0].Quote.TotalPaise != 1_634_400 {
		t.Fatalf("plantation room only %d, want ₹16,344", pr.Plans[0].Quote.TotalPaise)
	}

	nye, err := svc.Search(ctx, inventory.SearchRequest{CheckIn: day("2026-12-30"), CheckOut: day("2027-01-01"), Rooms: 1, Adults: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range nye {
		if o.Slug == "pool-villa" && o.Status != inventory.SoldOut {
			t.Fatalf("pool villa over New Year: %s", o.Status)
		}
	}

	alts, err := svc.Alternatives(ctx, lookup(t, ctx, pool, "pool-villa", "RO-FLEX").room, day("2026-12-30"), 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(alts) == 0 {
		t.Fatal("expected nearby dates for the pool villa")
	}
	for _, a := range alts {
		if !a.Before(day("2026-12-29")) && a.Before(day("2027-01-02")) {
			t.Errorf("suggested %s, which overlaps the sold-out nights", a.Format(time.DateOnly))
		}
	}
}

func TestAllotmentCannotDropBelowSoldOrHeld(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	svc := inventory.NewService(pool, today)
	cv := lookup(t, ctx, pool, "coffee-villa", "RO-FLEX")
	if _, err := svc.CreateHold(ctx, req(cv, "2026-11-21", "2026-11-22", 1, 2, 0)); err != nil {
		t.Fatal(err)
	}
	check := func(from, to string, n int) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			return inventory.CheckAllotment(ctx, tx, cv.room, day(from), day(to), n)
		})
	}
	var ae *inventory.AllotmentError
	if err := check("2026-11-14", "2026-11-16", 0); !errors.As(err, &ae) || ae.Sold != 1 {
		t.Fatalf("sold night: got %v", err)
	}
	if err := check("2026-11-21", "2026-11-22", 0); !errors.As(err, &ae) || ae.Held != 1 {
		t.Fatalf("held night: got %v", err)
	}
	if err := check("2026-11-17", "2026-11-19", 1); err != nil {
		t.Fatalf("free nights should allow a lower allotment: %v", err)
	}
}

// ---- helpers

func expire(t *testing.T, ctx context.Context, pool *pgxpool.Pool, holdID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE holds SET expires_at = now() - interval '1 second' WHERE id = $1`, holdID); err != nil {
		t.Fatal(err)
	}
}

func sold(t *testing.T, ctx context.Context, pool *pgxpool.Pool, room uuid.UUID, date string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT sold FROM night_inventory WHERE room_type_id = $1 AND date = $2`, room, date).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// assertNeverOversold checks every night: sold + live holds never exceeds the allotment,
// and sold always equals the nights actually booked.
func assertNeverOversold(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var bad int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM night_inventory ni
		LEFT JOIN LATERAL (SELECT coalesce(sum(rooms), 0) AS held FROM holds h
			WHERE h.room_type_id = ni.room_type_id AND h.status = 'ACTIVE' AND h.expires_at > now()
			  AND h.check_in <= ni.date AND h.check_out > ni.date) h ON true
		LEFT JOIN LATERAL (SELECT coalesce(sum(rooms), 0) AS booked FROM booking_nights bn
			WHERE bn.room_type_id = ni.room_type_id AND bn.date = ni.date) b ON true
		WHERE ni.sold + h.held > ni.allotment OR ni.sold <> b.booked`).Scan(&bad)
	if err != nil {
		t.Fatal(err)
	}
	if bad > 0 {
		t.Fatalf("%d nights are oversold or out of step with bookings", bad)
	}
}

func insertBooking(t *testing.T, ctx context.Context, pool *pgxpool.Pool, x ids, h inventory.Hold) uuid.UUID {
	t.Helper()
	id, ref := idgen.New(), idgen.BookingRef()
	_, err := pool.Exec(ctx, `INSERT INTO bookings (id, ref, hold_id, guest_name, guest_email, guest_phone, guest_country,
		adults, rooms, room_type_id, rate_plan_id, check_in, check_out, status, subtotal_paise, discount_paise, tax_paise, total_paise,
		price_breakdown, consent_version, consent_at)
		VALUES ($1,$2,$3,'Test Guest','t@example.test','+91 90000 00000','IN',2,1,$4,$5,$6,$7,'PENDING_PAYMENT',$8,$9,$10,$11,'{}','2026-10',now())`,
		id, ref, h.ID, x.room, x.plan, h.CheckIn, h.CheckOut, h.Quote.SubtotalPaise, h.Quote.DiscountPaise, h.Quote.TaxPaise, h.Quote.TotalPaise)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
