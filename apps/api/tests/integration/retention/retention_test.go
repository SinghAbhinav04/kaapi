package retention_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/retention"
	"kaapi/api/tests/testdb"
)

// "Today" is the date the seed data was written for.
var now = func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }

// confirmed returns the refs of n seeded confirmed bookings.
func confirmed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, n int) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT ref FROM bookings WHERE status = 'CONFIRMED' ORDER BY ref LIMIT $1`, n)
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, r)
	}
	if len(refs) != n {
		t.Fatalf("seed has %d confirmed bookings, want %d", len(refs), n)
	}
	return refs
}

func setStay(t *testing.T, ctx context.Context, pool *pgxpool.Pool, ref, in, out string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE bookings SET check_in = $2, check_out = $3 WHERE ref = $1`, ref, in, out); err != nil {
		t.Fatal(err)
	}
}

type row struct {
	exists, archived, anonymised bool
	email                        string
}

func lookup(t *testing.T, ctx context.Context, pool *pgxpool.Pool, ref string) row {
	t.Helper()
	var r row
	err := pool.QueryRow(ctx, `SELECT true, archived_at IS NOT NULL, anonymised_at IS NOT NULL, guest_email FROM bookings WHERE ref = $1`, ref).
		Scan(&r.exists, &r.archived, &r.anonymised, &r.email)
	if err != nil && err.Error() != "no rows in result set" {
		t.Fatal(err)
	}
	return r
}

func TestSweepArchivesAnonymisesAndPurges(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	refs := confirmed(t, ctx, pool, 4)
	recent, yearOld, taxExpired, taxKept := refs[0], refs[1], refs[2], refs[3]
	setStay(t, ctx, pool, recent, "2026-10-28", "2026-10-30")     // over, but inside 12 months
	setStay(t, ctx, pool, yearOld, "2025-09-01", "2025-09-03")    // over 12 months ago
	setStay(t, ctx, pool, taxExpired, "2017-05-01", "2017-05-03") // FY 2017-18 + 8 years ended 31 Mar 2026
	setStay(t, ctx, pool, taxKept, "2018-05-01", "2018-05-03")    // FY 2018-19 is kept until 31 Mar 2027

	// An old enquiry with a draft: both should go. A fresh one stays.
	var oldEnq, freshEnq string
	if err := pool.QueryRow(ctx, `SELECT ref FROM enquiries ORDER BY ref LIMIT 1`).Scan(&oldEnq); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT ref FROM enquiries ORDER BY ref DESC LIMIT 1`).Scan(&freshEnq); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE enquiries SET created_at = '2025-10-01' WHERE ref = $1`, oldEnq); err != nil {
		t.Fatal(err)
	}

	var soldBefore int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(sold), 0) FROM night_inventory`).Scan(&soldBefore); err != nil {
		t.Fatal(err)
	}

	svc := retention.NewService(pool, now)
	res, err := svc.Sweep(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if r := lookup(t, ctx, pool, recent); !r.archived || r.anonymised {
		t.Errorf("recent stay: archived=%v anonymised=%v, want archived only", r.archived, r.anonymised)
	}
	if r := lookup(t, ctx, pool, yearOld); !r.archived || !r.anonymised || r.email == "" || r.email[len(r.email)-len(".invalid"):] != ".invalid" {
		t.Errorf("year-old stay: %+v, want archived and anonymised with an .invalid email", r)
	}
	if r := lookup(t, ctx, pool, taxExpired); r.exists {
		t.Errorf("stay past the tax window still exists")
	}
	if r := lookup(t, ctx, pool, taxKept); !r.exists || !r.archived {
		t.Errorf("stay inside the tax window: %+v, want kept and archived", r)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM enquiries WHERE ref = $1`, oldEnq).Scan(&n); err != nil || n != 0 {
		t.Errorf("old enquiry: count=%d err=%v, want deleted", n, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM enquiries WHERE ref = $1`, freshEnq).Scan(&n); err != nil || n != 1 {
		t.Errorf("fresh enquiry: count=%d err=%v, want kept", n, err)
	}

	// Inventory is never touched: past nights' sold counts are history, not something to free up.
	var soldAfter int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(sold), 0) FROM night_inventory`).Scan(&soldAfter); err != nil {
		t.Fatal(err)
	}
	if soldAfter != soldBefore {
		t.Errorf("sold changed from %d to %d", soldBefore, soldAfter)
	}

	if res.PurgedTax != 1 || res.Anonymised < 1 || res.Archived < 3 || res.Enquiries != 1 {
		t.Errorf("result %+v", res)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'retention.sweep'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("audit rows = %d, err %v; want 1", n, err)
	}

	// Idempotent: a second run right away changes nothing.
	again, err := svc.Sweep(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.Total() != 0 {
		t.Errorf("second sweep changed %+v", again)
	}
}
