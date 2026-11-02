package inventory

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/pricing"
)

// HoldRequest asks for rooms on a rate plan for a stay.
type HoldRequest struct {
	RoomTypeID uuid.UUID
	RatePlanID uuid.UUID
	domain.Stay
	domain.Party
	PromoCode string
}

// Hold is a short reservation while the guest checks out.
type Hold struct {
	ID         uuid.UUID
	Token      string // shown once to the guest's browser; only its hash is stored
	RoomTypeID uuid.UUID
	RatePlanID uuid.UUID
	domain.Stay
	Rooms     int
	ExpiresAt time.Time
	Quote     pricing.Quote
}

// SearchRequest asks what can be booked for a stay.
type SearchRequest struct {
	domain.Stay
	domain.Party
	PromoCode string
}

// Status is a room type's availability for a search.
type Status string

const (
	Available     Status = "AVAILABLE"
	SoldOut       Status = "SOLD_OUT"
	Restricted    Status = "RESTRICTED"
	TooManyGuests Status = "TOO_MANY_GUESTS"
	NotYetOnSale  Status = "NOT_ON_SALE"
)

// Option is one room type in the search results.
type Option struct {
	RoomTypeID    uuid.UUID
	Slug, Name    string
	Status        Status
	Left          int // fewest rooms free on any night of the stay, website allotment only
	Reason        string
	SoldOutNights []time.Time
	MinStay       int
	Plans         []PlanQuote
	PromoError    string // set when the code doesn't apply to this room; prices are shown without it
}

// PlanQuote is one rate plan's price for the stay.
type PlanQuote struct {
	PlanID     uuid.UUID
	Code, Name string
	Refundable bool
	Quote      pricing.Quote
}

// Querier is satisfied by both a pool and a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// settings is the slice of property_settings this package needs.
type settings struct {
	location    *time.Location
	holdMinutes int
	maxNights   int
	maxRooms    int
	slabs       []pricing.Slab
}

// night is one locked night_inventory row, plus the rooms held by live holds.
type night struct {
	date              time.Time
	allotment, sold   int
	held              int
	stopSell          bool
	stopSellReason    *string
	minStay           int
	closedToArrival   bool
	closedToDeparture bool
}

// nightAvail is one night as seen by search, without locking.
type nightAvail struct {
	date          time.Time
	left, minStay int
	stopSell, cta bool
}

const (
	// holdTokenBytes is the size of the secret a guest's browser keeps for its hold.
	holdTokenBytes = 32
	// alternativesWindowDays and maxAlternatives bound the "try these dates" suggestions.
	alternativesWindowDays = 7
	maxAlternatives        = 3
)
