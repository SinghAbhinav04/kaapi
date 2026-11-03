package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
)

// store is every query this package runs.
type store struct{}

const messageColumns = `id, channel, kind, status, recipient, subject, body, created_at`

func (store) list(ctx context.Context, q db.Querier, limit int) ([]Message, error) {
	rows, err := q.Query(ctx, `SELECT `+messageColumns+` FROM outbox_messages ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) { return scanMessage(r) })
}

func (store) insertStaffMessage(ctx context.Context, q db.Querier, actor uuid.UUID, c Compose, status domain.OutboxStatus, externalID *string) (Message, error) {
	return scanMessage(q.QueryRow(ctx, `INSERT INTO outbox_messages (id, channel, kind, status, recipient, subject, body, sent_by, external_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+messageColumns,
		idgen.New(), domain.OutboxEmail, domain.OutboxStaffMessage, status, pii.Text(c.To), c.Subject, pii.Text(c.Body), actor, externalID))
}

func scanMessage(r pgx.Row) (Message, error) {
	var m Message
	return m, r.Scan(&m.ID, &m.Channel, &m.Kind, &m.Status, pii.Into(&m.Recipient), &m.Subject, pii.Into(&m.Body), &m.CreatedAt)
}

// claim takes up to limit queued emails that are due and not being sent by another worker
// (or whose worker gave up), marking them claimed. SKIP LOCKED lets several workers share
// the queue without waiting on each other.
func (store) claim(ctx context.Context, q db.Querier, limit int) ([]queued, error) {
	rows, err := q.Query(ctx, `UPDATE outbox_messages SET claimed_at = now(), attempts = attempts + 1
		WHERE id IN (SELECT id FROM outbox_messages
			WHERE status = $1 AND channel = $2 AND next_attempt_at <= now()
			  AND (claimed_at IS NULL OR claimed_at < now() - make_interval(secs => $3))
			ORDER BY next_attempt_at LIMIT $4 FOR UPDATE SKIP LOCKED)
		RETURNING id, recipient, coalesce(subject, ''), body, attempts`,
		domain.OutboxQueued, domain.OutboxEmail, claimTimeout.Seconds(), limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (queued, error) {
		var m queued
		return m, r.Scan(&m.ID, pii.Into(&m.Recipient), &m.Subject, pii.Into(&m.Body), &m.Attempts)
	})
}

func (store) markSent(ctx context.Context, q db.Querier, id uuid.UUID, status domain.OutboxStatus, externalID *string) error {
	_, err := q.Exec(ctx, `UPDATE outbox_messages SET status = $2, external_id = $3, sent_at = now(), claimed_at = NULL, last_error = NULL
		WHERE id = $1`, id, status, externalID)
	return err
}

// markFailed puts a message back in the queue after wait, or gives up when final.
func (store) markFailed(ctx context.Context, q db.Querier, id uuid.UUID, final bool, wait time.Duration, reason string) error {
	status := domain.OutboxQueued
	if final {
		status = domain.OutboxFailed
	}
	_, err := q.Exec(ctx, `UPDATE outbox_messages SET status = $2, claimed_at = NULL, last_error = $3,
		next_attempt_at = now() + make_interval(secs => $4) WHERE id = $1`, id, status, reason, wait.Seconds())
	return err
}
