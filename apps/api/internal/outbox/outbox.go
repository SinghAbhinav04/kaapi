// Package outbox records every message the system "sends". Email and WhatsApp are
// simulated in this build, so staff (and reviewers) read them here.
package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
)

type Message struct {
	ID        uuid.UUID
	Channel   domain.OutboxChannel
	Kind      domain.OutboxKind
	Recipient string
	Subject   *string
	Body      string
	CreatedAt time.Time
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// List returns the newest messages first.
func (s *Service) List(ctx context.Context, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, channel, kind, recipient, subject, body, created_at
		FROM outbox_messages ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) {
		var m Message
		return m, r.Scan(&m.ID, &m.Channel, &m.Kind, &m.Recipient, &m.Subject, &m.Body, &m.CreatedAt)
	})
}
