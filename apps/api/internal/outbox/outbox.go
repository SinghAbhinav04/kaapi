// Package outbox records every message the system sends. Until a mailbox is connected,
// email and WhatsApp are simulated, so staff (and reviewers) read them here; once Gmail is
// connected, email goes out for real and the copy here says so.
package outbox

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/idgen"
)

type Message struct {
	ID        uuid.UUID
	Channel   domain.OutboxChannel
	Kind      domain.OutboxKind
	Status    domain.OutboxStatus
	Recipient string
	Subject   *string
	Body      string
	CreatedAt time.Time
}

// Compose is a new email written by staff, not a reply to an enquiry.
type Compose struct {
	To      string
	Subject string
	Body    string
}

// Mailer delivers email through the connected mailbox (internal/gmail).
type Mailer interface {
	Connected(ctx context.Context) bool
	Send(ctx context.Context, m domain.Mail) (domain.SentMail, error)
}

const (
	DefaultLimit   = 50
	MaxLimit       = 200
	maxSubjectLen  = 200
	maxBodyLen     = 4000
	messageColumns = `id, channel, kind, status, recipient, subject, body, created_at`
)

type Service struct {
	pool   *pgxpool.Pool
	mailer Mailer // nil: messages are simulated
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// SetMailer connects real delivery; the API does it once at start-up.
func (s *Service) SetMailer(m Mailer) { s.mailer = m }

// List returns the newest messages first.
func (s *Service) List(ctx context.Context, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+messageColumns+` FROM outbox_messages ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) { return scan(r) })
}

// Send delivers a staff-written email. With a mailbox connected it goes out for real (the
// network call happens before anything is written, outside any transaction); otherwise it
// is recorded as a simulation.
func (s *Service) Send(ctx context.Context, actor uuid.UUID, c Compose) (Message, error) {
	c.To = strings.ToLower(strings.TrimSpace(c.To))
	c.Subject = strings.Join(strings.Fields(c.Subject), " ")
	c.Body = strings.TrimSpace(c.Body)
	fields := map[string]string{}
	if a, err := mail.ParseAddress(c.To); err != nil || a.Address != c.To {
		fields["to"] = "must be one email address"
	}
	if c.Subject == "" || len([]rune(c.Subject)) > maxSubjectLen {
		fields["subject"] = "must be 1 to 200 characters"
	}
	if c.Body == "" || len([]rune(c.Body)) > maxBodyLen {
		fields["body"] = "must be 1 to 4000 characters"
	}
	if len(fields) > 0 {
		return Message{}, apperr.Validation(fields)
	}

	status, sent := domain.OutboxSimulated, domain.SentMail{}
	if s.mailer != nil && s.mailer.Connected(ctx) {
		var err error
		if sent, err = s.mailer.Send(ctx, domain.Mail{To: c.To, Subject: c.Subject, Body: c.Body}); err != nil {
			return Message{}, err
		}
		status = domain.OutboxSent
	}
	var external *string
	if sent.MessageID != "" {
		external = &sent.MessageID
	}
	row := s.pool.QueryRow(context.WithoutCancel(ctx), `INSERT INTO outbox_messages (id, channel, kind, status, recipient, subject, body, sent_by, external_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+messageColumns,
		idgen.New(), domain.OutboxEmail, domain.OutboxStaffMessage, status, c.To, c.Subject, c.Body, actor, external)
	return scan(row)
}

func scan(r pgx.Row) (Message, error) {
	var m Message
	return m, r.Scan(&m.ID, &m.Channel, &m.Kind, &m.Status, &m.Recipient, &m.Subject, &m.Body, &m.CreatedAt)
}
