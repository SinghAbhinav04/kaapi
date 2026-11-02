package admin

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
)

// Night is one cell of the rates grid.
type Night struct {
	Date           time.Time
	PricePaise     int64 // the selected plan's price, one room, before GST
	Allotment      int
	Sold           int
	Held           int
	Left           int
	StopSell       bool
	StopSellReason *string
	MinStay        int
	NoArrival      bool
}

// Row is one room type across the grid's nights.
type Row struct {
	RoomTypeID uuid.UUID
	Slug       string
	Name       string
	TotalKeys  int
	Nights     []Night
}

// Grid is the rates and allotment screen.
type Grid struct {
	Plan string
	domain.Stay
	Rows []Row
}

// Change is one bulk edit from the staff console. Nil fields are left as they are.
type Change struct {
	RoomTypeID uuid.UUID
	domain.Stay
	Weekdays           []time.Weekday // empty: every night in the range
	RoomOnlyPricePaise *int64         // other plans are derived from it
	Allotment          *int
	StopSell           *bool
	StopSellReason     *string
	MinStay            *int
	NoArrival          *bool
}

// Touches reports which kinds of fields the change edits, for permission checks.
func (c Change) TouchesPrice() bool { return c.RoomOnlyPricePaise != nil }
func (c Change) TouchesInventory() bool {
	return c.Allotment != nil || c.StopSell != nil || c.MinStay != nil || c.NoArrival != nil
}

// Limits for staff edits.
const (
	RoomOnlyPlan      = "RO-FLEX"
	maxGridDays       = 62
	maxEditDays       = 180
	maxMinStay        = 14
	maxPricePaise     = 10_00_000_00 // ₹10 lakh a night: anything above is a typo
	planRoundingPaise = 10_000       // derived plan prices round to ₹100
	stopSellReasonMax = 120
	auditActionRates  = "rates.bulk_update"
	auditEntityRoom   = "room_type"
)
