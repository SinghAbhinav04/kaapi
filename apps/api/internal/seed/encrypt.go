package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/platform/pii"
)

// encryptBatch bounds one transaction, so a large table is encrypted without long locks.
const encryptBatch = 200

// sealedLike matches stored ciphertext ("pii:v1:…"), to find rows still in the clear.
const sealedLike = "pii:%"

// EncryptExisting encrypts personal data written before encryption existed and fills the
// blind indexes (migration 0012 adds the columns; the database can't do this itself, since
// the key never reaches it). It walks each table once in id order, so it always ends, and
// skips rows already done, so running it on every start is cheap and safe.
func EncryptExisting(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	total := 0
	for _, step := range []struct {
		name string
		run  func(context.Context, pgx.Tx, uuid.UUID) (uuid.UUID, int, error)
	}{
		{"users", encryptUsers}, {"bookings", encryptBookings}, {"enquiries", encryptEnquiries}, {"outbox", encryptOutbox},
	} {
		after := uuid.Nil
		for {
			var last uuid.UUID
			var n int
			err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
				var err error
				last, n, err = step.run(ctx, tx, after)
				return err
			})
			if err != nil {
				return total, fmt.Errorf("encrypt %s: %w", step.name, err)
			}
			total += n
			if last == uuid.Nil {
				break
			}
			after = last
		}
	}
	return total, nil
}

// Each step reads the next batch after `after` that still needs work and returns the last id
// it saw (uuid.Nil when the table is done) and how many rows it encrypted. Scanning into
// pii.Text decrypts what is already sealed and passes plaintext through, so every row is
// simply written back sealed.

func encryptUsers(ctx context.Context, tx pgx.Tx, after uuid.UUID) (uuid.UUID, int, error) {
	rows, err := tx.Query(ctx, `SELECT id, email, name, phone FROM users
		WHERE id > $1 AND (email_hash IS NULL OR email NOT LIKE $2 OR name NOT LIKE $2 OR (phone <> '' AND phone NOT LIKE $2))
		ORDER BY id LIMIT $3 FOR UPDATE`, after, sealedLike, encryptBatch)
	if err != nil {
		return uuid.Nil, 0, err
	}
	type row struct {
		id          uuid.UUID
		email, name pii.Text
		phone       pii.NullText
	}
	batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.email, &x.name, &x.phone)
	})
	if err != nil || len(batch) == 0 {
		return uuid.Nil, 0, err
	}
	for _, x := range batch {
		if _, err := tx.Exec(ctx, `UPDATE users SET email = $2, email_hash = $3, name = $4, phone = $5 WHERE id = $1`,
			x.id, x.email, pii.EmailIndex(string(x.email)), x.name, x.phone); err != nil {
			return uuid.Nil, 0, err
		}
	}
	return batch[len(batch)-1].id, len(batch), nil
}

func encryptBookings(ctx context.Context, tx pgx.Tx, after uuid.UUID) (uuid.UUID, int, error) {
	// An anonymised booking keeps its erased+…@example.invalid placeholder plain and unindexed.
	rows, err := tx.Query(ctx, `SELECT id, guest_name, guest_email, guest_phone, guest_country, anonymised_at IS NOT NULL FROM bookings
		WHERE id > $1 AND (guest_name NOT LIKE $2 OR guest_country NOT LIKE $2 OR (guest_phone <> '' AND guest_phone NOT LIKE $2)
			OR (anonymised_at IS NULL AND (guest_email_hash IS NULL OR guest_email NOT LIKE $2)))
		ORDER BY id LIMIT $3 FOR UPDATE`, after, sealedLike, encryptBatch)
	if err != nil {
		return uuid.Nil, 0, err
	}
	type row struct {
		id                          uuid.UUID
		name, email, phone, country pii.Text
		anonymised                  bool
	}
	batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.name, &x.email, &x.phone, &x.country, &x.anonymised)
	})
	if err != nil || len(batch) == 0 {
		return uuid.Nil, 0, err
	}
	for _, x := range batch {
		var email any = x.email
		var hash *string
		if x.anonymised {
			email = string(x.email) // the placeholder, not personal data
		} else {
			h := pii.EmailIndex(string(x.email))
			hash = &h
		}
		if _, err := tx.Exec(ctx, `UPDATE bookings SET guest_name = $2, guest_email = $3, guest_email_hash = $4, guest_phone = $5,
			guest_country = $6 WHERE id = $1`, x.id, x.name, email, hash, x.phone, x.country); err != nil {
			return uuid.Nil, 0, err
		}
	}
	return batch[len(batch)-1].id, len(batch), nil
}

func encryptEnquiries(ctx context.Context, tx pgx.Tx, after uuid.UUID) (uuid.UUID, int, error) {
	rows, err := tx.Query(ctx, `SELECT id, sender_name, sender_email, sender_phone FROM enquiries
		WHERE id > $1 AND ((sender_hash IS NULL AND (sender_email IS NOT NULL OR sender_phone IS NOT NULL))
			OR sender_name NOT LIKE $2 OR sender_email NOT LIKE $2 OR sender_phone NOT LIKE $2)
		ORDER BY id LIMIT $3 FOR UPDATE`, after, sealedLike, encryptBatch)
	if err != nil {
		return uuid.Nil, 0, err
	}
	type row struct {
		id                 uuid.UUID
		name, email, phone pii.NullText
	}
	batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.name, &x.email, &x.phone)
	})
	if err != nil || len(batch) == 0 {
		return uuid.Nil, 0, err
	}
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	for _, x := range batch {
		if _, err := tx.Exec(ctx, `UPDATE enquiries SET sender_name = $2, sender_email = $3, sender_phone = $4, sender_hash = $5 WHERE id = $1`,
			x.id, x.name, x.email, x.phone, enquiry.SenderIndex(deref(x.email.Text), deref(x.phone.Text))); err != nil {
			return uuid.Nil, 0, err
		}
	}
	return batch[len(batch)-1].id, len(batch), nil
}

func encryptOutbox(ctx context.Context, tx pgx.Tx, after uuid.UUID) (uuid.UUID, int, error) {
	rows, err := tx.Query(ctx, `SELECT id, recipient, body FROM outbox_messages
		WHERE id > $1 AND (recipient NOT LIKE $2 OR (body <> '' AND body NOT LIKE $2))
		ORDER BY id LIMIT $3 FOR UPDATE`, after, sealedLike, encryptBatch)
	if err != nil {
		return uuid.Nil, 0, err
	}
	type row struct {
		id              uuid.UUID
		recipient, body pii.Text
	}
	batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.recipient, &x.body)
	})
	if err != nil || len(batch) == 0 {
		return uuid.Nil, 0, err
	}
	for _, x := range batch {
		if _, err := tx.Exec(ctx, `UPDATE outbox_messages SET recipient = $2, body = $3 WHERE id = $1`, x.id, x.recipient, x.body); err != nil {
			return uuid.Nil, 0, err
		}
	}
	return batch[len(batch)-1].id, len(batch), nil
}
