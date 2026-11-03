package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
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
	DefaultLimit  = 50
	MaxLimit      = 200
	maxSubjectLen = 200
	maxBodyLen    = 4000
)
