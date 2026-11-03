package outbox

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
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
		idgen.New(), domain.OutboxEmail, domain.OutboxStaffMessage, status, c.To, c.Subject, c.Body, actor, externalID))
}

func scanMessage(r pgx.Row) (Message, error) {
	var m Message
	return m, r.Scan(&m.ID, &m.Channel, &m.Kind, &m.Status, &m.Recipient, &m.Subject, &m.Body, &m.CreatedAt)
}
