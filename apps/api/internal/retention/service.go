// Package retention archives finished bookings, strips guest contact details once they are
// no longer needed, and deletes what the privacy policy says we don't keep. It runs as a
// periodic job; every step is idempotent and works in small batches with SKIP LOCKED, so it
// never blocks bookings or the inbox and can be re-run at any time.
package retention

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{pool: pool, now: now}
}

// Sweep runs every rule once and records what it did in the audit log.
func (s *Service) Sweep(ctx context.Context) (Result, error) {
	var r Result
	now := s.now()
	today := todayIST(now)
	var err error

	steps := []struct {
		count *int64
		run   func() (int64, error)
	}{
		{&r.Archived, func() (int64, error) { return s.archive(ctx, today, now) }},
		{&r.Anonymised, func() (int64, error) { return s.anonymise(ctx, today.AddDate(0, -contactRetention, 0), now) }},
		{&r.PurgedTax, func() (int64, error) {
			return s.purgeBookings(ctx, `SELECT id FROM bookings WHERE archived_at IS NOT NULL AND check_out < $1`, PurgeBefore(today, TaxRetentionFY))
		}},
		{&r.PurgedUnpaid, func() (int64, error) {
			return s.purgeBookings(ctx, `SELECT id FROM bookings b WHERE b.status = ANY($1) AND b.created_at < $2
				AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.booking_id = b.id AND p.status = $3)`,
				[]string{string(domain.BookingExpired), string(domain.BookingPaymentFailed)}, now.Add(-unpaidRetention), domain.PaymentCaptured)
		}},
		// Enquiry events, drafts and alerts cascade; outbox copies lose the link and go below.
		{&r.Enquiries, func() (int64, error) {
			return s.deleteByID(ctx, "enquiries", `SELECT id FROM enquiries WHERE created_at < $1`, now.AddDate(0, -messageRetention, 0))
		}},
		{&r.Outbox, func() (int64, error) {
			return s.deleteByID(ctx, "outbox_messages", `SELECT id FROM outbox_messages WHERE created_at < $1`, now.AddDate(0, -messageRetention, 0))
		}},
		{&r.Notifications, func() (int64, error) {
			return s.deleteByID(ctx, "notifications", `SELECT id FROM notifications WHERE created_at < $1`, now.AddDate(0, -messageRetention, 0))
		}},
		{&r.Holds, func() (int64, error) {
			return s.deleteByID(ctx, "holds", `SELECT h.id FROM holds h WHERE h.status <> $1 AND h.expires_at < $2
				AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.hold_id = h.id)`, domain.HoldActive, now.Add(-holdRetention))
		}},
		{&r.OTPs, func() (int64, error) {
			return s.deleteByID(ctx, "otp_codes", `SELECT id FROM otp_codes WHERE expires_at < $1`, now.Add(-otpRetention))
		}},
		{&r.Sessions, func() (int64, error) { return s.purgeSessions(ctx, now.Add(-sessionRetention)) }},
		{&r.Housekeeping, func() (int64, error) { return s.housekeeping(ctx, now) }},
	}
	for _, step := range steps {
		if *step.count, err = step.run(); err != nil {
			return r, err
		}
	}
	if r.Total() > 0 {
		err = s.audit(ctx, today, r)
	}
	return r, err
}

// archive marks bookings whose stay is over. They leave the live list but keep every link.
func (s *Service) archive(ctx context.Context, today, now time.Time) (int64, error) {
	statuses := []string{string(domain.BookingConfirmed), string(domain.BookingCancelled), string(domain.BookingRefunded)}
	return s.batches(ctx, `SELECT id FROM bookings WHERE archived_at IS NULL AND status = ANY($1) AND check_out < $2`,
		[]any{statuses, today}, func(tx pgx.Tx, ids []uuid.UUID) error {
			_, err := tx.Exec(ctx, `UPDATE bookings SET archived_at = $2, updated_at = $2 WHERE id = ANY($1)`, ids, now)
			return err
		})
}

// anonymise wipes contact details from archived bookings. The email becomes a reserved
// .invalid address, so the column stays NOT NULL and "My bookings" can no longer find it.
func (s *Service) anonymise(ctx context.Context, before, now time.Time) (int64, error) {
	return s.batches(ctx, `SELECT id FROM bookings WHERE archived_at IS NOT NULL AND anonymised_at IS NULL AND check_out < $1`,
		[]any{before}, func(tx pgx.Tx, ids []uuid.UUID) error {
			_, err := tx.Exec(ctx, `UPDATE bookings SET guest_email = 'erased+' || lower(ref) || '@example.invalid', guest_phone = '',
				special_requests = NULL, arrival_window = NULL, guest_user_id = NULL, marketing_opt_in = false,
				anonymised_at = $2, updated_at = $2 WHERE id = ANY($1)`, ids, now)
			return err
		})
}

// purgeBookings deletes bookings with their payments and holds. Nights and alerts cascade;
// outbox copies keep their text but lose the link.
func (s *Service) purgeBookings(ctx context.Context, pick string, args ...any) (int64, error) {
	return s.batches(ctx, pick, args, func(tx pgx.Tx, ids []uuid.UUID) error {
		for _, q := range []string{
			`UPDATE enquiries SET linked_booking_id = NULL WHERE linked_booking_id = ANY($1)`,
			`DELETE FROM payments WHERE booking_id = ANY($1)`,
		} {
			if _, err := tx.Exec(ctx, q, ids); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `WITH gone AS (DELETE FROM bookings WHERE id = ANY($1) RETURNING hold_id)
			DELETE FROM holds WHERE id IN (SELECT hold_id FROM gone WHERE hold_id IS NOT NULL)`, ids)
		return err
	})
}

// purgeSessions deletes long-dead sessions. A session can point at the one that replaced
// it, so those links are cleared first.
func (s *Service) purgeSessions(ctx context.Context, before time.Time) (int64, error) {
	return s.batches(ctx, `SELECT id FROM sessions WHERE expires_at < $1 OR revoked_at < $1`, []any{before},
		func(tx pgx.Tx, ids []uuid.UUID) error {
			if _, err := tx.Exec(ctx, `UPDATE sessions SET replaced_by = NULL WHERE replaced_by = ANY($1)`, ids); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, ids)
			return err
		})
}

// housekeeping clears small tables keyed without ids; they hold no personal data worth batching for.
func (s *Service) housekeeping(ctx context.Context, now time.Time) (int64, error) {
	var n int64
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		n = 0
		a, err := tx.Exec(ctx, `DELETE FROM rate_limit_buckets WHERE window_start < $1`, now.Add(-bucketRetention))
		if err != nil {
			return err
		}
		b, err := tx.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at < $1`, now)
		if err != nil {
			return err
		}
		n = a.RowsAffected() + b.RowsAffected()
		return nil
	})
	return n, err
}

func (s *Service) deleteByID(ctx context.Context, table, pick string, args ...any) (int64, error) {
	del := `DELETE FROM ` + table + ` WHERE id = ANY($1)` // table names are constants in this file
	return s.batches(ctx, pick, args, func(tx pgx.Tx, ids []uuid.UUID) error {
		_, err := tx.Exec(ctx, del, ids)
		return err
	})
}

// batches locks up to batchSize matching rows (skipping any another transaction holds),
// acts on them and commits, until a batch comes back short.
func (s *Service) batches(ctx context.Context, pick string, args []any, act func(pgx.Tx, []uuid.UUID) error) (int64, error) {
	q := pick + ` ORDER BY id LIMIT ` + strconv.Itoa(batchSize) + ` FOR UPDATE SKIP LOCKED`
	var total int64
	for {
		var n int
		err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, q, args...)
			if err != nil {
				return err
			}
			ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			n = len(ids)
			if n == 0 {
				return nil
			}
			return act(tx, ids)
		})
		if err != nil {
			return total, err
		}
		total += int64(n)
		if n < batchSize {
			return total, nil
		}
	}
}

func (s *Service) audit(ctx context.Context, today time.Time, r Result) error {
	after, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_logs (id, action, entity, entity_id, after) VALUES ($1, $2, $3, $4, $5)`,
		idgen.New(), auditAction, auditEntity, today.Format(time.DateOnly), after)
	return err
}

// todayIST is the resort's calendar date, as stay dates are stored: midnight UTC.
func todayIST(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
