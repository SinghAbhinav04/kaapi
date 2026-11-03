package pii_test

import (
	"strings"
	"testing"

	"kaapi/api/internal/platform/pii"
	"kaapi/api/tests/testdb"
)

// The interceptor must really encrypt through pgx: a named string type could otherwise be
// sent as its underlying string, storing plaintext without any error.
func TestTextIsEncryptedInTheDatabaseAndDecryptedOnRead(t *testing.T) {
	if err := pii.Configure([]byte("test-only-pii-key-0123456789abcdef")); err != nil {
		t.Fatal(err)
	}
	ctx, p := testdb.Seeded(t)
	pool, err := p.Acquire(ctx) // one connection: the probe table is temporary
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Release()
	if _, err := pool.Exec(ctx, `CREATE TEMP TABLE pii_probe (v text, n text)`); err != nil {
		t.Fatal(err)
	}
	name := "Meera Iyer"
	if _, err := pool.Exec(ctx, `INSERT INTO pii_probe (v, n) VALUES ($1, $2)`, pii.Text("guest@example.test"), pii.NullText{Text: &name}); err != nil {
		t.Fatal(err)
	}
	var rawV, rawN string
	if err := pool.QueryRow(ctx, `SELECT v, n FROM pii_probe`).Scan(&rawV, &rawN); err != nil {
		t.Fatal(err)
	}
	if !pii.IsSealed(rawV) || !pii.IsSealed(rawN) || strings.Contains(rawV, "guest") || strings.Contains(rawN, "Meera") {
		t.Fatalf("stored in the clear: %q %q", rawV, rawN)
	}
	var v pii.Text
	var n pii.NullText
	if err := pool.QueryRow(ctx, `SELECT v, n FROM pii_probe`).Scan(&v, &n); err != nil {
		t.Fatal(err)
	}
	if v != "guest@example.test" || n.Text == nil || *n.Text != name {
		t.Fatalf("round trip: %q %v", v, n.Text)
	}
}
