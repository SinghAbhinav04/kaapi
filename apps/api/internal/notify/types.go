package notify

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
)

// Notification is one alert in the console: a complaint, an enquiry the AI couldn't sort, a
// late payment that needs a refund.
type Notification struct {
	ID         uuid.UUID
	Type       domain.NotificationType
	Body       string
	EnquiryRef *string
	BookingRef *string
	CreatedAt  time.Time
	ReadAt     *time.Time
}

// Viewer is who is asking: alerts go to a team (role) or a person, and administrators see all.
type Viewer struct {
	User uuid.UUID
	Team domain.Role
}

func (v Viewer) seesAll() bool { return v.Team == domain.RoleAdmin }

// Page is the newest alerts and how many are unread.
type Page struct {
	Items  []Notification
	Unread int
}

const (
	DefaultLimit = 30
	MaxLimit     = 100
)

var ErrNotFound = apperr.New(apperr.CodeNotFound, "That alert doesn't exist.")
