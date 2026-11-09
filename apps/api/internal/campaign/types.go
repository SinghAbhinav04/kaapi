package campaign

import (
	"time"

	"github.com/google/uuid"
)

// Kind of campaign: an offer staff published, a festive season, or an announcement.
type Kind string

const (
	KindOffer        Kind = "OFFER"
	KindFestive      Kind = "FESTIVE"
	KindAnnouncement Kind = "ANNOUNCEMENT"
)

// Status of a campaign.
type Status string

const (
	StatusScheduled Status = "SCHEDULED"
	StatusSending   Status = "SENDING"
	StatusSent      Status = "SENT"
)

// Campaign is one email to every opted-in guest.
type Campaign struct {
	ID         uuid.UUID
	Kind       Kind
	Subject    string
	Body       string
	Status     Status
	SendAt     time.Time
	SentAt     *time.Time
	Recipients int
	CreatedAt  time.Time
}

// Input is a campaign staff write.
type Input struct {
	Kind    Kind
	Subject string
	Body    string
	SendAt  *time.Time // nil: now
}

// Offer is what the offers service announces when it publishes one (see admin.OfferObserver).
type Offer struct {
	ID          uuid.UUID
	Code        string
	Description string
	StayFrom    time.Time
	StayTo      time.Time
	BookTo      time.Time
	MinNights   int
	AutoApply   bool // applies by itself at checkout; no code to type
}

const (
	// fanOutBatch bounds each transaction; the outbox then sends at its own pace (Gmail allows
	// a few hundred a day from one mailbox, so big lists go out over time, never all at once).
	fanOutBatch = 200
	subjectMax  = 150
	bodyMax     = 4000
)
