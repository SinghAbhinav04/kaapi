package retention

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

// store is every query the sweep runs. pick* methods lock up to batchSize matching rows,
// skipping any another transaction holds; the matching act methods change them.
type store struct{}

var (
	archivable = []string{string(domain.BookingConfirmed), string(domain.BookingCancelled), string(domain.BookingRefunded)}
	unpaid     = []string{string(domain.BookingExpired), string(domain.BookingPaymentFailed)}
)

func (store) pick(ctx context.Context, q db.Querier, where string, args ...any) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, where+` ORDER BY id LIMIT `+strconv.Itoa(batchSize)+` FOR UPDATE SKIP LOCKED`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (s store) pickToArchive(ctx context.Context, q db.Querier, today time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM bookings WHERE archived_at IS NULL AND status = ANY($1) AND check_out < $2`, archivable, today)
}

func (s store) pickToAnonymise(ctx context.Context, q db.Querier, checkedOutBefore time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM bookings WHERE archived_at IS NOT NULL AND anonymised_at IS NULL AND check_out < $1`, checkedOutBefore)
}

func (s store) pickPastTaxWindow(ctx context.Context, q db.Querier, checkedOutBefore time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM bookings WHERE archived_at IS NOT NULL AND check_out < $1`, checkedOutBefore)
}

func (s store) pickUnpaid(ctx context.Context, q db.Querier, createdBefore time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM bookings b WHERE b.status = ANY($1) AND b.created_at < $2
		AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.booking_id = b.id AND p.status = $3)`, unpaid, createdBefore, domain.PaymentCaptured)
}

func (s store) pickEnquiries(ctx context.Context, q db.Querier, before time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM enquiries WHERE created_at < $1`, before)
}

func (s store) pickOutbox(ctx context.Context, q db.Querier, before time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM outbox_messages WHERE created_at < $1`, before)
}

func (s store) pickNotifications(ctx context.Context, q db.Querier, before time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM notifications WHERE created_at < $1`, before)
}

func (s store) pickHolds(ctx context.Context, q db.Querier, expiredBefore time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT h.id FROM holds h WHERE h.status <> $1 AND h.expires_at < $2
		AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.hold_id = h.id)`, domain.HoldActive, expiredBefore)
}

func (s store) pickOTPs(ctx context.Context, q db.Querier, expiredBefore time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM otp_codes WHERE expires_at < $1`, expiredBefore)
}

func (s store) pickSessions(ctx context.Context, q db.Querier, before time.Time) ([]uuid.UUID, error) {
	return s.pick(ctx, q, `SELECT id FROM sessions WHERE expires_at < $1 OR revoked_at < $1`, before)
}

func (store) archive(ctx context.Context, q db.Querier, ids []uuid.UUID, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE bookings SET archived_at = $2, updated_at = $2 WHERE id = ANY($1)`, ids, now)
	return err
}

// anonymise wipes contact details. The email becomes a reserved .invalid address, so the
// column stays NOT NULL and "My bookings" can no longer find it.
func (store) anonymise(ctx context.Context, q db.Querier, ids []uuid.UUID, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE bookings SET guest_email = 'erased+' || lower(ref) || '@example.invalid', guest_phone = '',
		special_requests = NULL, arrival_window = NULL, guest_user_id = NULL, marketing_opt_in = false,
		anonymised_at = $2, updated_at = $2 WHERE id = ANY($1)`, ids, now)
	return err
}

// deleteBookings removes bookings with their payments and holds. Nights and alerts cascade;
// outbox copies keep their text but lose the link.
func (store) deleteBookings(ctx context.Context, q db.Querier, ids []uuid.UUID) error {
	for _, sql := range []string{
		`UPDATE enquiries SET linked_booking_id = NULL WHERE linked_booking_id = ANY($1)`,
		`DELETE FROM payments WHERE booking_id = ANY($1)`,
		`WITH gone AS (DELETE FROM bookings WHERE id = ANY($1) RETURNING hold_id)
			DELETE FROM holds WHERE id IN (SELECT hold_id FROM gone WHERE hold_id IS NOT NULL)`,
	} {
		if _, err := q.Exec(ctx, sql, ids); err != nil {
			return err
		}
	}
	return nil
}

// deleteFrom removes rows by id. table is always one of this file's constants.
func (store) deleteFrom(ctx context.Context, q db.Querier, table string, ids []uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM `+table+` WHERE id = ANY($1)`, ids)
	return err
}

// deleteSessions clears replacement links first: a session can point at the one that replaced it.
func (store) deleteSessions(ctx context.Context, q db.Querier, ids []uuid.UUID) error {
	if _, err := q.Exec(ctx, `UPDATE sessions SET replaced_by = NULL WHERE replaced_by = ANY($1)`, ids); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, ids)
	return err
}

// housekeeping clears small tables keyed without ids; they hold no personal data worth batching for.
func (store) housekeeping(ctx context.Context, q db.Querier, bucketsBefore, now time.Time) (int64, error) {
	a, err := q.Exec(ctx, `DELETE FROM rate_limit_buckets WHERE window_start < $1`, bucketsBefore)
	if err != nil {
		return 0, err
	}
	b, err := q.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return a.RowsAffected() + b.RowsAffected(), nil
}

func (store) audit(ctx context.Context, q db.Querier, today time.Time, r Result) error {
	after, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_logs (id, action, entity, entity_id, after) VALUES ($1, $2, $3, $4, $5)`,
		idgen.New(), auditAction, auditEntity, today.Format(time.DateOnly), after)
	return err
}
