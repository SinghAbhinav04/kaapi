package ai

import (
	"context"
	"time"

	"kaapi/api/internal/domain"
)

// BudgetBasis says whether a budget is per night or for the whole stay or event.
type BudgetBasis string

const (
	BudgetPerNight BudgetBasis = "PER_NIGHT"
	BudgetTotal    BudgetBasis = "TOTAL"
)

// Extracted is what the classifier pulled out of a message. Every field is optional.
type Extracted struct {
	CheckIn     *time.Time
	CheckOut    *time.Time
	Adults      *int
	Children    *int
	Rooms       *int
	GuestCount  *int // events: total guests
	BudgetINR   *int64
	BudgetBasis *BudgetBasis
	EventMonth  *string // YYYY-MM
}

// Classification is one classifier's reading of an enquiry.
type Classification struct {
	Category   domain.Category
	Secondary  *domain.Category
	Confidence float64 // 0..1, the classifier's own estimate
	Language   string  // en, hi, hinglish, kn-en, de …
	Summary    string
	Extracted  Extracted
}

// DraftInput is everything a reply draft may use. Prices are never passed: the server
// fills {{QUOTE}} after the model has written the prose.
type DraftInput struct {
	Message   string // redacted
	Category  domain.Category
	Extracted Extracted
	Facts     []Fact
	StaffName string
	Today     time.Time
}

type Fact struct {
	Question string
	Answer   string
}

// Provider is an LLM that can classify and draft. Gemini implements it; tests use a fake.
type Provider interface {
	Name() string
	Classify(ctx context.Context, redactedMessage string, today time.Time) (Classification, error)
	Draft(ctx context.Context, in DraftInput) (string, error)
}

// Placeholders a draft may contain. The server replaces them; anything else in braces is removed.
const (
	PlaceholderGuestName = "{{GUEST_NAME}}"
	PlaceholderQuote     = "{{QUOTE}}"
	PlaceholderEventTeam = "{{EVENT_TEAM_CONTACT}}"
	PlaceholderSignature = "{{SIGNATURE}}"
)

// Tuning. The unsorted threshold is chosen from the evaluation report (cmd/eval).
const (
	PromptVersion        = "2026-11-03.1"
	UnsortedBelow        = 0.6
	agreementBonus       = 0.05
	disagreementPenalty  = 0.25
	missingFieldsPenalty = 0.1
	maxMessageRunes      = 4000
	maxGuests            = 2000
	maxNightsInMessage   = 60
)
