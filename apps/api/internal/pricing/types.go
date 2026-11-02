package pricing

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
)

// Slab is one GST band. MaxTariffPaise is the per-room, per-night ceiling for the band;
// nil means "everything above the previous band".
type Slab struct {
	MaxTariffPaise *int64 `json:"max_tariff_paise_per_night"`
	RateBps        int64  `json:"rate_bps"`
}

// Night is the plan price for one room on one night, before extra guests, discount and GST.
type Night struct {
	Date       time.Time
	PricePaise int64
}

// Room holds the occupancy rules of a room type.
type Room struct {
	BaseOccupancy   int
	MaxAdults       int
	MaxChildren     int
	ExtraAdultPaise int64
}

// Promo is a discount that has already passed eligibility checks (see Eligible).
type Promo struct {
	Code  string
	Type  domain.PromoType
	Value int64 // basis points for PERCENT, paise for FLAT
}

// PromoRule is a promo code as stored, with the limits that decide whether it applies.
type PromoRule struct {
	Promo
	StayFrom, StayTo time.Time
	BookFrom, BookTo time.Time
	MinNights        int
	MaxUses          *int
	UsedCount        int
	RoomTypeIDs      []uuid.UUID // empty = any room type
	Active           bool
}

type Input struct {
	Nights []Night
	domain.Party
	Room  Room
	Promo *Promo
	Slabs []Slab
}

// Line is one night of the quote, for all rooms together.
type Line struct {
	Date          time.Time `json:"date"`
	RoomPaise     int64     `json:"room_paise"`     // plan price × rooms + extra adults
	DiscountPaise int64     `json:"discount_paise"` // promo share for this night
	TaxablePaise  int64     `json:"taxable_paise"`
	GSTRateBps    int64     `json:"gst_rate_bps"`
	GSTPaise      int64     `json:"gst_paise"`
	TotalPaise    int64     `json:"total_paise"`
}

type Quote struct {
	Lines         []Line `json:"lines"`
	SubtotalPaise int64  `json:"subtotal_paise"`
	DiscountPaise int64  `json:"discount_paise"`
	TaxPaise      int64  `json:"tax_paise"`
	TotalPaise    int64  `json:"total_paise"`
	PromoCode     string `json:"promo_code,omitempty"`
}

// Basis points: 10,000 bps = 100%.
const bpsWhole = 10_000
