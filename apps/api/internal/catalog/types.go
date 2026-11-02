package catalog

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
)

type Photo struct {
	URL string
	Alt string
}

type Plan struct {
	ID                 uuid.UUID
	Code               string
	Name               string
	Inclusions         []string
	Refundable         bool
	CancellationPolicy domain.CancellationPolicy
	MinNights          int
}

type RoomType struct {
	ID              uuid.UUID
	Slug            string
	Name            string
	Kind            domain.RoomKind
	SizeSqm         int
	Bed             string
	View            string
	BaseOccupancy   int
	MaxAdults       int
	MaxChildren     int
	ExtraAdultPaise int64
	Summary         string
	Description     string
	Amenities       []string
	Photos          []Photo
	Plans           []Plan
}

// CalendarDay is one night on a room's price calendar.
type CalendarDay struct {
	Date       time.Time
	PricePaise int64 // the plan's price for one room, before GST
	Left       int   // website rooms still free (allotment − sold − live holds)
	MinStay    int
	StopSell   bool
	NoArrival  bool
	Season     bool // priced above the plain weekday/weekend rate (a season uplift applies)
}

type Property struct {
	Name         string
	CheckInTime  string
	CheckOutTime string
	HoldMinutes  int
	Facts        []Fact
}

type Fact struct {
	Key      string
	Category string
	Question string
	Answer   string
}

// maxCalendarDays bounds a calendar request (a little over one month view).
const maxCalendarDays = 42
