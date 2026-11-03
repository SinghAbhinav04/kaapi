package booking

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/pricing"
)

// Guest is who the booking is for. Only what's needed to confirm and contact them: no
// passport or ID details online (Form C details are taken at check-in).
type Guest struct {
	Name    string
	Email   string
	Phone   string
	Country string // ISO 3166 alpha-2, e.g. IN
}

type CreateInput struct {
	HoldID          uuid.UUID
	HoldToken       string
	Guest           Guest
	ArrivalWindow   string
	SpecialRequests string
	Consent         bool // DPDP: the guest agreed to their details being used for this booking
	MarketingOptIn  bool
	GuestUserID     *uuid.UUID // set when the guest is signed in
	// DisplayCurrency is what the guest was reading prices in (empty: not sent). It is kept
	// with that moment's rate for analytics; the charge is INR either way.
	DisplayCurrency domain.Currency
}

type Booking struct {
	ID           uuid.UUID
	Ref          string
	Status       domain.BookingStatus
	RoomTypeID   uuid.UUID
	RoomTypeName string
	RoomTypeSlug string
	RatePlanName string
	Refundable   bool
	domain.Stay
	Rooms           int
	Adults          int
	Children        int
	Guest           Guest
	ArrivalWindow   *string
	SpecialRequests *string
	Quote           pricing.Quote
	HoldExpiresAt   *time.Time
	CreatedAt       time.Time
	ConfirmedAt     *time.Time
	ArchivedAt      *time.Time // set by the retention sweep once the stay is over
	Payment         *Payment
}

type Payment struct {
	OrderID       string
	Status        domain.PaymentStatus
	AmountPaise   int64
	FailureReason *string
}

// ListFilter narrows the staff bookings list.
type ListFilter struct {
	Status   domain.BookingStatus // empty: any
	From, To time.Time            // check-in range; zero: any
	Archived bool                 // false: live bookings; true: the archive
	Limit    int
}

// Field limits and policy constants.
const (
	ConsentVersion      = "2026-10"
	nameMin, nameMax    = 2, 100
	phoneMinDigits      = 10
	phoneMaxDigits      = 15
	requestsMax         = 1000
	arrivalWindowMax    = 40
	refAttempts         = 5
	DefaultListLimit    = 100
	MaxListLimit        = 500
	cancellationNoticeD = 7
)
