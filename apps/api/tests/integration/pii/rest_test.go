package pii_test

import (
	"testing"

	"github.com/google/uuid"

	"kaapi/api/internal/platform/pii"
	"kaapi/api/internal/seed"
	"kaapi/api/tests/testdb"
)

// Everything personal the seed writes is ciphertext in the database, with its blind index.
func TestPersonalDataIsCiphertextAtRest(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	checks := []struct{ name, query string }{
		{"users", `SELECT count(*) FROM users WHERE email NOT LIKE 'pii:v1:%' OR name NOT LIKE 'pii:v1:%' OR email_hash IS NULL`},
		{"bookings", `SELECT count(*) FROM bookings WHERE guest_name NOT LIKE 'pii:v1:%' OR guest_email NOT LIKE 'pii:v1:%'
			OR guest_phone NOT LIKE 'pii:v1:%' OR guest_country NOT LIKE 'pii:v1:%' OR guest_email_hash IS NULL`},
		{"enquiries", `SELECT count(*) FROM enquiries WHERE sender_name NOT LIKE 'pii:v1:%' OR sender_email NOT LIKE 'pii:v1:%'
			OR sender_phone NOT LIKE 'pii:v1:%' OR sender_hash IS NULL`},
	}
	for _, c := range checks {
		var plain int
		if err := pool.QueryRow(ctx, c.query).Scan(&plain); err != nil || plain != 0 {
			t.Errorf("%s: %d rows with personal data in the clear (%v)", c.name, plain, err)
		}
	}
	var guest pii.Text
	if err := pool.QueryRow(ctx, `SELECT email FROM users WHERE email_hash = $1`, pii.EmailIndex("guest@example.test")).Scan(&guest); err != nil || guest != "guest@example.test" {
		t.Fatalf("lookup by blind index, then decrypt: %q %v", guest, err)
	}
}

// A database from before encryption: plaintext rows are encrypted and indexed on start, and a
// second run finds nothing to do.
func TestBackfillEncryptsOldRowsOnce(t *testing.T) {
	ctx, pool := testdb.Seeded(t)
	id := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, phone, role) VALUES ($1, 'old.guest@example.test', 'Old Guest', '+91 90000 30003', 'GUEST')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, recipient, subject, body)
		VALUES ($1, 'EMAIL', 'OTP', 'plain@example.test', 'Your code', 'plain body')`, uuid.New()); err != nil {
		t.Fatal(err)
	}
	n, err := seed.EncryptExisting(ctx, pool)
	if err != nil || n < 1 {
		t.Fatalf("backfill: %d %v", n, err)
	}
	var rawEmail, hash string
	var name pii.Text
	if err := pool.QueryRow(ctx, `SELECT email, email_hash, name FROM users WHERE id = $1`, id).Scan(&rawEmail, &hash, &name); err != nil {
		t.Fatal(err)
	}
	if !pii.IsSealed(rawEmail) || hash != pii.EmailIndex("old.guest@example.test") || name != "Old Guest" {
		t.Fatalf("after backfill: %q %q %q", rawEmail, hash, name)
	}
	var plainOutbox int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_messages WHERE recipient NOT LIKE 'pii:v1:%'`).Scan(&plainOutbox); err != nil || plainOutbox != 0 {
		t.Fatalf("outbox still in the clear: %d %v", plainOutbox, err)
	}
	if again, err := seed.EncryptExisting(ctx, pool); err != nil || again != 0 {
		t.Fatalf("second run should find nothing: %d %v", again, err)
	}
}
