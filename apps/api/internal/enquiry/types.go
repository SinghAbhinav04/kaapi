package enquiry

import (
	"context"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
)

// IntakeInput is a message arriving from any channel.
type IntakeInput struct {
	Channel     domain.Channel
	SenderName  string
	SenderEmail string
	SenderPhone string
	Subject     string
	Text        string
	External    *External // set for mail imported from the connected mailbox
}

// External identifies an imported email: it's stored once, and replies thread under it.
type External struct {
	ID           string // the provider's message id
	ThreadID     string
	RFCMessageID string
}

// Mailer delivers replies for real once a mailbox is connected (internal/gmail). Without
// one, sending records the reply in the outbox as a simulation, as before.
type Mailer interface {
	Connected(ctx context.Context) bool
	Send(ctx context.Context, m domain.Mail) (domain.SentMail, error)
}

// Summary is one row of the inbox.
type Summary struct {
	Ref        string
	Channel    domain.Channel
	SenderName *string
	Snippet    string
	ReceivedAt time.Time
	Category   *domain.Category
	Confidence *float64
	Lane       domain.Lane
	Status     domain.EnquiryStatus
	Priority   domain.Priority
	AIStatus   domain.AIStatus
	NeedsCheck bool // the AI is unsure, or a stay enquiry has no dates
	HasDraft   bool
	FollowUps  int // the guest's later messages in the thread
}

// Enquiry is the full record for the detail pane.
type Enquiry struct {
	ID          uuid.UUID
	Ref         string
	Channel     domain.Channel
	SenderName  *string
	SenderEmail *string
	SenderPhone *string
	Subject     *string
	Text        string
	ReceivedAt  time.Time
	Language    *string
	Category    *domain.Category
	Secondary   *domain.Category
	Confidence  *float64
	Extracted   ai.Extracted
	Summary     *string
	Lane        domain.Lane
	AssignedTo  *Staff
	Priority    domain.Priority
	Status      domain.EnquiryStatus
	AIStatus    domain.AIStatus
	AIModel     *string
	AIReason    string
	FirstReply  *time.Time
	Draft       *Draft
	FollowUps   []FollowUp // the guest's later messages in the thread, oldest first
	Events      []Event
}

// FollowUp is a later message from the guest on the same enquiry (a reply in its thread).
type FollowUp struct {
	Text       string
	ReceivedAt time.Time
}

type Staff struct {
	ID   uuid.UUID
	Name string
	Role domain.Role
}

type Draft struct {
	Version       int
	Body          string // what will be sent
	Template      string
	Author        string
	FlaggedReason *string
	QuoteFilled   bool
	CreatedAt     time.Time
	SentAt        *time.Time
	SignOff       string // the sender's once sent, else the viewer's: Send adds it below Body
}

type Event struct {
	Type      domain.EnquiryEventType
	Actor     domain.Actor
	ActorName *string
	From      map[string]any
	To        map[string]any
	CreatedAt time.Time
}

type ListFilter struct {
	Lanes  []domain.Lane // nil: every lane; empty: none
	Status domain.EnquiryStatus
	Query  string
	Limit  int
}

// Corrections are staff fixes to what the AI extracted.
type Corrections struct {
	CheckIn, CheckOut *time.Time
	Adults, Children  *int
	Rooms, GuestCount *int
}

// Stats are the numbers on the staff dashboard, tied to the brief's success measures.
type Stats struct {
	MedianFirstReplyMinutes *float64 // target: under 30 in working hours
	RepliedThisWeek         int
	OpenByLane              map[domain.Lane]int
	OpenComplaints          int
	AwaitingAI              int
	AICorrectionRate        *float64 // share of AI-routed enquiries staff re-routed this week
	DirectBookingsThisWeek  int
	DirectRevenueThisWeek   int64
	Overbookings            int // always 0: the database refuses them
}

const (
	DefaultListLimit   = 100
	MaxListLimit       = 300
	snippetRunes       = 140
	snippetSourceChars = 400 // read this much of each message for the list's snippet
	maxDraftRunes      = 4000
	intakePerHour      = 5 // per sender, against spam floods
	dedupeWindow       = 24 * time.Hour
	firstRefNumber     = 4410
	refAttempts        = 5
	workerBatch        = 10
	maxQuoteOptions    = 3
	statsWindow        = 7 * 24 * time.Hour
	defaultQuoteAdult  = 2
	// sendClaimTTL: a delivery that crashed mid-flight frees its draft after this long.
	sendClaimTTL = 2 * time.Minute
)

// Offer kinds as promo_codes stores them; a percentage is held in basis points.
const (
	offerPercent  = "PERCENT"
	bpsPerPercent = 100
)

// alertRoles are emailed about every complaint: the GM owns it, and the administrators (the
// owner) hear of it too, so a complaint reaches a real person even with no GM on the team.
var alertRoles = []domain.Role{domain.RoleGM, domain.RoleAdmin}

// alertEmailWindow: complaints received longer ago than this are alerted in the console only.
// Emails are for news; a backlog worked through after an outage, or seeded data, isn't.
const alertEmailWindow = 24 * time.Hour

// senderIndexKind keeps the sender's blind index apart from other hashed identifiers.
const senderIndexKind = "sender"
