package api

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/catalog"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/pricing"
)

// JSON shapes of the public API. Money is integer paise, dates are YYYY-MM-DD strings,
// timestamps are RFC 3339. contract/openapi.yaml describes the same shapes, and the
// contract test checks real responses against it.

type PhotoDTO struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type PlanDTO struct {
	ID                 uuid.UUID                 `json:"id"`
	Code               string                    `json:"code"`
	Name               string                    `json:"name"`
	Inclusions         []string                  `json:"inclusions"`
	Refundable         bool                      `json:"refundable"`
	CancellationPolicy domain.CancellationPolicy `json:"cancellation_policy"`
	MinNights          int                       `json:"min_nights"`
}

type SleepsDTO struct {
	Adults   int `json:"adults"`
	Children int `json:"children"`
}

type RoomDTO struct {
	ID          uuid.UUID       `json:"id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Kind        domain.RoomKind `json:"kind"`
	SizeSqm     int             `json:"size_sqm"`
	Bed         string          `json:"bed"`
	View        string          `json:"view"`
	Sleeps      SleepsDTO       `json:"sleeps"`
	ExtraAdult  int64           `json:"extra_adult_paise"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	Amenities   []string        `json:"amenities"`
	Photos      []PhotoDTO      `json:"photos"`
	Plans       []PlanDTO       `json:"plans"`
}

type RoomsResponse struct {
	Rooms []RoomDTO `json:"rooms"`
}

type RoomResponse struct {
	Room RoomDTO `json:"room"`
}

type CalendarDayDTO struct {
	Date       string `json:"date"`
	PricePaise int64  `json:"price_paise"`
	Left       int    `json:"left"`
	MinStay    int    `json:"min_stay"`
	StopSell   bool   `json:"stop_sell"`
	NoArrival  bool   `json:"no_arrival"`
	Season     bool   `json:"season"`
}

type CalendarResponse struct {
	Plan string           `json:"plan"`
	Days []CalendarDayDTO `json:"days"`
}

type QuoteLineDTO struct {
	Date          string `json:"date"`
	RoomPaise     int64  `json:"room_paise"`
	DiscountPaise int64  `json:"discount_paise"`
	TaxablePaise  int64  `json:"taxable_paise"`
	GSTRateBps    int64  `json:"gst_rate_bps"`
	GSTPaise      int64  `json:"gst_paise"`
	TotalPaise    int64  `json:"total_paise"`
}

type QuoteDTO struct {
	Lines         []QuoteLineDTO `json:"lines"`
	SubtotalPaise int64          `json:"subtotal_paise"`
	DiscountPaise int64          `json:"discount_paise"`
	TaxPaise      int64          `json:"tax_paise"`
	TotalPaise    int64          `json:"total_paise"`
	PromoCode     *string        `json:"promo_code"`
}

type StayDTO struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Nights   int    `json:"nights"`
}

type PlanQuoteDTO struct {
	PlanID     uuid.UUID `json:"plan_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Refundable bool      `json:"refundable"`
	Quote      QuoteDTO  `json:"quote"`
}

type OptionDTO struct {
	RoomTypeID    uuid.UUID        `json:"room_type_id"`
	Slug          string           `json:"slug"`
	Name          string           `json:"name"`
	Status        inventory.Status `json:"status"`
	Left          int              `json:"left"`
	Reason        *string          `json:"reason"`
	SoldOutNights []string         `json:"sold_out_nights"`
	MinStay       int              `json:"min_stay"`
	PromoError    *string          `json:"promo_error"`
	Plans         []PlanQuoteDTO   `json:"plans"`
}

type AvailabilityResponse struct {
	Stay    StayDTO     `json:"stay"`
	Options []OptionDTO `json:"options"`
}

type CreateHoldRequest struct {
	RoomTypeID uuid.UUID `json:"room_type_id"`
	RatePlanID uuid.UUID `json:"rate_plan_id"`
	CheckIn    string    `json:"check_in"`
	CheckOut   string    `json:"check_out"`
	Rooms      int       `json:"rooms"`
	Adults     int       `json:"adults"`
	Children   int       `json:"children"`
	PromoCode  string    `json:"promo_code"`
}

type HoldDTO struct {
	ID         uuid.UUID `json:"id"`
	Token      string    `json:"token,omitempty"` // only in the create response
	RoomTypeID uuid.UUID `json:"room_type_id"`
	RatePlanID uuid.UUID `json:"rate_plan_id"`
	Stay       StayDTO   `json:"stay"`
	Rooms      int       `json:"rooms"`
	ExpiresAt  time.Time `json:"expires_at"`
	Quote      QuoteDTO  `json:"quote"`
}

type HoldResponse struct {
	Hold HoldDTO `json:"hold"`
}

type FactDTO struct {
	Key      string `json:"key"`
	Category string `json:"category"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type PropertyResponse struct {
	Name         string    `json:"name"`
	CheckInTime  string    `json:"check_in_time"`
	CheckOutTime string    `json:"check_out_time"`
	HoldMinutes  int       `json:"hold_minutes"`
	Facts        []FactDTO `json:"facts"`
}

// ---- mappers

func date(t time.Time) string { return t.Format(time.DateOnly) }

func stayDTO(s domain.Stay) StayDTO {
	return StayDTO{CheckIn: date(s.CheckIn), CheckOut: date(s.CheckOut), Nights: s.Nights()}
}

func quoteDTO(q pricing.Quote) QuoteDTO {
	out := QuoteDTO{Lines: make([]QuoteLineDTO, len(q.Lines)), SubtotalPaise: q.SubtotalPaise,
		DiscountPaise: q.DiscountPaise, TaxPaise: q.TaxPaise, TotalPaise: q.TotalPaise, PromoCode: optional(q.PromoCode)}
	for i, l := range q.Lines {
		out.Lines[i] = QuoteLineDTO{Date: date(l.Date), RoomPaise: l.RoomPaise, DiscountPaise: l.DiscountPaise,
			TaxablePaise: l.TaxablePaise, GSTRateBps: l.GSTRateBps, GSTPaise: l.GSTPaise, TotalPaise: l.TotalPaise}
	}
	return out
}

func roomDTO(r catalog.RoomType) RoomDTO {
	out := RoomDTO{ID: r.ID, Slug: r.Slug, Name: r.Name, Kind: r.Kind, SizeSqm: r.SizeSqm, Bed: r.Bed, View: r.View,
		Sleeps: SleepsDTO{Adults: r.MaxAdults, Children: r.MaxChildren}, ExtraAdult: r.ExtraAdultPaise,
		Summary: r.Summary, Description: r.Description, Amenities: nonNil(r.Amenities),
		Photos: make([]PhotoDTO, len(r.Photos)), Plans: make([]PlanDTO, len(r.Plans))}
	for i, p := range r.Photos {
		out.Photos[i] = PhotoDTO{URL: p.URL, Alt: p.Alt}
	}
	for i, p := range r.Plans {
		out.Plans[i] = PlanDTO{ID: p.ID, Code: p.Code, Name: p.Name, Inclusions: nonNil(p.Inclusions), Refundable: p.Refundable,
			CancellationPolicy: p.CancellationPolicy, MinNights: p.MinNights}
	}
	return out
}

func optionDTO(o inventory.Option) OptionDTO {
	out := OptionDTO{RoomTypeID: o.RoomTypeID, Slug: o.Slug, Name: o.Name, Status: o.Status, Left: o.Left,
		Reason: optional(o.Reason), SoldOutNights: domain.FormatDates(o.SoldOutNights), MinStay: o.MinStay,
		PromoError: optional(o.PromoError), Plans: make([]PlanQuoteDTO, len(o.Plans))}
	for i, p := range o.Plans {
		out.Plans[i] = PlanQuoteDTO{PlanID: p.PlanID, Code: p.Code, Name: p.Name, Refundable: p.Refundable, Quote: quoteDTO(p.Quote)}
	}
	return out
}

func holdDTO(h inventory.Hold, withToken bool) HoldDTO {
	out := HoldDTO{ID: h.ID, RoomTypeID: h.RoomTypeID, RatePlanID: h.RatePlanID, Stay: stayDTO(h.Stay),
		Rooms: h.Rooms, ExpiresAt: h.ExpiresAt, Quote: quoteDTO(h.Quote)}
	if withToken {
		out.Token = h.Token
	}
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
