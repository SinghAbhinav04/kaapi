package api

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/payment"
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

// ---- auth

type UserDTO struct {
	ID          uuid.UUID         `json:"id"`
	Email       string            `json:"email"`
	Name        string            `json:"name"`
	Role        domain.Role       `json:"role"`
	Permissions []auth.Permission `json:"permissions"`
}

type AuthResponse struct {
	User UserDTO `json:"user"`
	// Tokens are only in the body for the native app (X-Client: native); the web app
	// gets httpOnly cookies that scripts can't read.
	AccessToken     *string    `json:"access_token,omitempty"`
	AccessExpiresAt *time.Time `json:"access_expires_at,omitempty"`
	RefreshToken    *string    `json:"refresh_token,omitempty"`
}

type StaffLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OTPRequest struct {
	Email string `json:"email"`
}

type OTPRequestResponse struct {
	Sent     bool    `json:"sent"`
	DemoCode *string `json:"demo_code,omitempty"` // only when DEMO_MODE is on, so reviewers can sign in
}

type OTPVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type OutboxMessageDTO struct {
	ID        uuid.UUID            `json:"id"`
	Channel   domain.OutboxChannel `json:"channel"`
	Kind      domain.OutboxKind    `json:"kind"`
	Recipient string               `json:"recipient"`
	Subject   *string              `json:"subject"`
	Body      string               `json:"body"`
	CreatedAt time.Time            `json:"created_at"`
}

type OutboxResponse struct {
	Messages []OutboxMessageDTO `json:"messages"`
}

// ---- bookings and payments

type GuestDTO struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Country string `json:"country"`
}

type CreateBookingRequest struct {
	HoldID          uuid.UUID `json:"hold_id"`
	Guest           GuestDTO  `json:"guest"`
	ArrivalWindow   string    `json:"arrival_window"`
	SpecialRequests string    `json:"special_requests"`
	Consent         bool      `json:"consent"`
	MarketingOptIn  bool      `json:"marketing_opt_in"`
}

type RoomRefDTO struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type PlanRefDTO struct {
	Name       string `json:"name"`
	Refundable bool   `json:"refundable"`
}

type PaymentDTO struct {
	OrderID       string               `json:"order_id"`
	Status        domain.PaymentStatus `json:"status"`
	AmountPaise   int64                `json:"amount_paise"`
	FailureReason *string              `json:"failure_reason"`
}

type BookingDTO struct {
	Ref             string               `json:"ref"`
	Status          domain.BookingStatus `json:"status"`
	Room            RoomRefDTO           `json:"room"`
	Plan            PlanRefDTO           `json:"plan"`
	Stay            StayDTO              `json:"stay"`
	Rooms           int                  `json:"rooms"`
	Adults          int                  `json:"adults"`
	Children        int                  `json:"children"`
	Guest           GuestDTO             `json:"guest"`
	ArrivalWindow   *string              `json:"arrival_window"`
	SpecialRequests *string              `json:"special_requests"`
	Quote           QuoteDTO             `json:"quote"`
	HoldExpiresAt   *time.Time           `json:"hold_expires_at"`
	CreatedAt       time.Time            `json:"created_at"`
	ConfirmedAt     *time.Time           `json:"confirmed_at"`
	Payment         *PaymentDTO          `json:"payment"`
}

type BookingResponse struct {
	Booking BookingDTO `json:"booking"`
}

type BookingsResponse struct {
	Bookings []BookingDTO `json:"bookings"`
}

// PaymentPageDTO is what the mock gateway page needs: no guest contact details.
type PaymentPageDTO struct {
	OrderID       string               `json:"order_id"`
	AmountPaise   int64                `json:"amount_paise"`
	Status        domain.PaymentStatus `json:"status"`
	FailureReason *string              `json:"failure_reason"`
	BookingRef    string               `json:"booking_ref"`
	BookingStatus domain.BookingStatus `json:"booking_status"`
	RoomName      string               `json:"room_name"`
	PlanName      string               `json:"plan_name"`
	Stay          StayDTO              `json:"stay"`
	HoldExpiresAt *time.Time           `json:"hold_expires_at"`
}

type SimulatePaymentRequest struct {
	Outcome payment.Outcome `json:"outcome"`
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

func bookingDTO(b booking.Booking) BookingDTO {
	out := BookingDTO{Ref: b.Ref, Status: b.Status, Room: RoomRefDTO{ID: b.RoomTypeID, Slug: b.RoomTypeSlug, Name: b.RoomTypeName},
		Plan: PlanRefDTO{Name: b.RatePlanName, Refundable: b.Refundable}, Stay: stayDTO(b.Stay), Rooms: b.Rooms,
		Adults: b.Adults, Children: b.Children,
		Guest:         GuestDTO{Name: b.Guest.Name, Email: b.Guest.Email, Phone: b.Guest.Phone, Country: b.Guest.Country},
		ArrivalWindow: b.ArrivalWindow, SpecialRequests: b.SpecialRequests, Quote: quoteDTO(b.Quote),
		HoldExpiresAt: b.HoldExpiresAt, CreatedAt: b.CreatedAt, ConfirmedAt: b.ConfirmedAt}
	if b.Payment != nil {
		out.Payment = &PaymentDTO{OrderID: b.Payment.OrderID, Status: b.Payment.Status, AmountPaise: b.Payment.AmountPaise,
			FailureReason: b.Payment.FailureReason}
	}
	return out
}

func paymentPageDTO(b booking.Booking) PaymentPageDTO {
	out := PaymentPageDTO{BookingRef: b.Ref, BookingStatus: b.Status, RoomName: b.RoomTypeName, PlanName: b.RatePlanName,
		Stay: stayDTO(b.Stay), HoldExpiresAt: b.HoldExpiresAt}
	if b.Payment != nil {
		out.OrderID, out.AmountPaise, out.Status, out.FailureReason = b.Payment.OrderID, b.Payment.AmountPaise, b.Payment.Status, b.Payment.FailureReason
	}
	return out
}
