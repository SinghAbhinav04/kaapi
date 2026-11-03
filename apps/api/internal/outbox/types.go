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

// Delivery of queued guest emails (Deliver). A failed send waits a little longer each time;
// after maxAttempts it is marked FAILED for staff to see in the outbox.
const (
	DeliverBatch = 20
	maxAttempts  = 5
	firstBackoff = 30 * time.Second
	// claimTimeout frees a message whose worker died mid-send. Gmail answers well within it,
	// so a claim this old means nobody is sending it any more.
	claimTimeout = 2 * time.Minute
	maxErrorLen  = 500
)

// queued is a message claimed for delivery.
type queued struct {
	ID        uuid.UUID
	Recipient string
	Subject   string
	Body      string
	Attempts  int
}

// DeliverResult counts one delivery run.
type DeliverResult struct{ Sent, Simulated, Retrying, Failed int }

func (r DeliverResult) Total() int { return r.Sent + r.Simulated + r.Retrying + r.Failed }
