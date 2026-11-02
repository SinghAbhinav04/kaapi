package seed_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/tests/testdb"
)

func TestSeededDataMatchesTheDesign(t *testing.T) {
	ctx, pool := testdb.Seeded(t)

	var enc string
	if err := pool.QueryRow(ctx, `SHOW server_encoding`).Scan(&enc); err != nil || enc != "UTF8" {
		t.Fatalf("encoding %q: Hindi enquiries and ₹ need UTF8", enc)
	}

	// Coffee Villa room-only: Diwali Friday, weekend Saturday, plain Sunday.
	wantPrices := map[string]int64{"2026-11-06": 2_420_000, "2026-11-14": 2_010_000, "2026-11-15": 1_750_000}
	for date, want := range wantPrices {
		var got int64
		err := pool.QueryRow(ctx, `SELECT nr.price_paise FROM night_rates nr
			JOIN rate_plans rp ON rp.id = nr.rate_plan_id JOIN room_types rt ON rt.id = rp.room_type_id
			WHERE rt.slug = 'coffee-villa' AND rp.code = 'RO-FLEX' AND nr.date = $1`, date).Scan(&got)
		if err != nil || got != want {
			t.Errorf("coffee villa %s: %d (%v), want %d", date, got, err, want)
		}
	}

	// ENQ-4411's dates: one villa left. ENQ-4421's New Year dates: pool villa sold out.
	assertLeft(t, ctx, pool, "coffee-villa", "2026-11-14", 1)
	assertLeft(t, ctx, pool, "pool-villa", "2026-12-31", 0)

	var hindi string
	if err := pool.QueryRow(ctx, `SELECT raw_text FROM enquiries WHERE ref = 'ENQ-4462'`).Scan(&hindi); err != nil || !strings.Contains(hindi, "शादी") {
		t.Errorf("Hindi enquiry did not round-trip: %q %v", hindi, err)
	}

	var staylonger int
	if err := pool.QueryRow(ctx, `SELECT used_count FROM promo_codes WHERE code = 'STAYLONGER'`).Scan(&staylonger); err != nil || staylonger != 2 {
		t.Errorf("STAYLONGER used %d times, want 2 (two confirmed bookings)", staylonger)
	}
}

func TestDatabaseRefusesOversellAndBadTotals(t *testing.T) {
	ctx, pool := testdb.Seeded(t)

	_, err := pool.Exec(ctx, `UPDATE night_inventory ni SET sold = sold + 1 FROM room_types rt
		WHERE rt.id = ni.room_type_id AND rt.slug = 'pool-villa' AND ni.date = '2026-12-31'`)
	assertConstraint(t, err, "night_inventory_no_oversell")

	_, err = pool.Exec(ctx, `UPDATE bookings SET tax_paise = tax_paise + 1 WHERE ref = 'KS-2A7F4Q'`)
	assertConstraint(t, err, "bookings_total_adds_up")

	_, err = pool.Exec(ctx, `UPDATE promo_codes SET used_count = max_uses + 1 WHERE code = 'STAYLONGER'`)
	assertConstraint(t, err, "promo_codes_no_overuse")
}

func assertLeft(t *testing.T, ctx context.Context, pool *pgxpool.Pool, slug, date string, want int) {
	t.Helper()
	var left int
	err := pool.QueryRow(ctx, `SELECT ni.allotment - ni.sold FROM night_inventory ni
		JOIN room_types rt ON rt.id = ni.room_type_id WHERE rt.slug = $1 AND ni.date = $2`, slug, date).Scan(&left)
	if err != nil || left != want {
		t.Errorf("%s on %s: %d left (%v), want %d", slug, date, left, err, want)
	}
}

func assertConstraint(t *testing.T, err error, name string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != name {
		t.Errorf("want violation of %s, got %v", name, err)
	}
}
