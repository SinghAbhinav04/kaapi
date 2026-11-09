package httpapi

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/admin"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/pricing"
	"kaapi/api/internal/retention"
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

// StaffChallengeResponse: the password was right and a code was emailed.
type StaffChallengeResponse struct {
	CodeSent bool   `json:"code_sent"`
	SentTo   string `json:"sent_to"`
}

type StaffLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OTPRequest struct {
	Email string `json:"email"`
}

type OTPRequestResponse struct {
	Sent bool `json:"sent"`
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
	Status    domain.OutboxStatus  `json:"status"`
	Recipient string               `json:"recipient"`
	Subject   *string              `json:"subject"`
	Body      string               `json:"body"`
	CreatedAt time.Time            `json:"created_at"`
}

type NotificationDTO struct {
	ID         uuid.UUID               `json:"id"`
	Type       domain.NotificationType `json:"type"`
	Body       string                  `json:"body"`
	EnquiryRef *string                 `json:"enquiry_ref"`
	BookingRef *string                 `json:"booking_ref"`
	CreatedAt  time.Time               `json:"created_at"`
	ReadAt     *time.Time              `json:"read_at"`
}

type NotificationsResponse struct {
	Notifications []NotificationDTO `json:"notifications"`
	Unread        int               `json:"unread"`
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
	// DisplayCurrency: what the guest read prices in; analytics only, the charge is INR.
	DisplayCurrency domain.Currency `json:"display_currency"`
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
	ArchivedAt      *time.Time           `json:"archived_at"`
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

// ---- staff: rates and allotment

type RateNightDTO struct {
	Date           string  `json:"date"`
	PricePaise     int64   `json:"price_paise"`
	Allotment      int     `json:"allotment"`
	Sold           int     `json:"sold"`
	Held           int     `json:"held"`
	Left           int     `json:"left"`
	StopSell       bool    `json:"stop_sell"`
	StopSellReason *string `json:"stop_sell_reason"`
	MinStay        int     `json:"min_stay"`
	NoArrival      bool    `json:"no_arrival"`
}

type RateRowDTO struct {
	RoomTypeID uuid.UUID      `json:"room_type_id"`
	Slug       string         `json:"slug"`
	Name       string         `json:"name"`
	TotalKeys  int            `json:"total_keys"`
	Nights     []RateNightDTO `json:"nights"`
}

type RatesGridResponse struct {
	Plan string       `json:"plan"`
	Stay StayDTO      `json:"stay"`
	Rows []RateRowDTO `json:"rows"`
}

type UpdateRatesRequest struct {
	RoomTypeID         uuid.UUID `json:"room_type_id"`
	From               string    `json:"from"`
	To                 string    `json:"to"`
	Weekdays           []string  `json:"weekdays"`
	RoomOnlyPricePaise *int64    `json:"room_only_price_paise"`
	Allotment          *int      `json:"allotment"`
	StopSell           *bool     `json:"stop_sell"`
	StopSellReason     *string   `json:"stop_sell_reason"`
	MinStay            *int      `json:"min_stay"`
	NoArrival          *bool     `json:"no_arrival"`
}

type UpdateRatesResponse struct {
	NightsChanged int `json:"nights_changed"`
}

// ---- enquiries

type EventDetailsDTO struct {
	Type        string `json:"type"` // wedding, engagement, offsite, …
	Date        string `json:"date"`
	Month       string `json:"month"`
	GuestCount  int    `json:"guest_count"`
	Nights      int    `json:"nights"`
	BudgetPaise int64  `json:"budget_paise"`
}

type CreateEnquiryRequest struct {
	Name    string           `json:"name"`
	Email   string           `json:"email"`
	Phone   string           `json:"phone"`
	Message string           `json:"message"`
	Kind    string           `json:"kind"`
	Event   *EventDetailsDTO `json:"event"`
}

type EnquiryCreatedResponse struct {
	Ref string `json:"ref"`
}

type InboundMessageRequest struct {
	Channel     string `json:"channel"`
	SenderName  string `json:"sender_name"`
	SenderEmail string `json:"sender_email"`
	SenderPhone string `json:"sender_phone"`
	Subject     string `json:"subject"`
	Text        string `json:"text"`
}

type EnquirySummaryDTO struct {
	Ref        string               `json:"ref"`
	Channel    domain.Channel       `json:"channel"`
	SenderName *string              `json:"sender_name"`
	Snippet    string               `json:"snippet"`
	ReceivedAt time.Time            `json:"received_at"`
	Category   *domain.Category     `json:"category"`
	Confidence *float64             `json:"confidence"`
	Lane       domain.Lane          `json:"lane"`
	Status     domain.EnquiryStatus `json:"status"`
	Priority   domain.Priority      `json:"priority"`
	AIStatus   domain.AIStatus      `json:"ai_status"`
	NeedsCheck bool                 `json:"needs_check"`
	HasDraft   bool                 `json:"has_draft"`
	FollowUps  int                  `json:"follow_ups"`
}

type EnquiriesResponse struct {
	Enquiries []EnquirySummaryDTO `json:"enquiries"`
}

type ExtractedDTO struct {
	CheckIn     *string `json:"check_in"`
	CheckOut    *string `json:"check_out"`
	Nights      *int    `json:"nights"`
	Adults      *int    `json:"adults"`
	Children    *int    `json:"children"`
	Rooms       *int    `json:"rooms"`
	GuestCount  *int    `json:"guest_count"`
	BudgetPaise *int64  `json:"budget_paise"`
	BudgetBasis *string `json:"budget_basis"`
	EventMonth  *string `json:"event_month"`
}

type StaffRefDTO struct {
	ID   uuid.UUID   `json:"id"`
	Name string      `json:"name"`
	Role domain.Role `json:"role"`
}

type DraftDTO struct {
	Version       int        `json:"version"`
	Body          string     `json:"body"`
	Author        string     `json:"author"`
	FlaggedReason *string    `json:"flagged_reason"`
	QuoteFilled   bool       `json:"quote_filled"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
	SignOff       string     `json:"sign_off"`
}

type FollowUpDTO struct {
	Text       string    `json:"text"`
	ReceivedAt time.Time `json:"received_at"`
}

type EnquiryEventDTO struct {
	Type      domain.EnquiryEventType `json:"type"`
	Actor     domain.Actor            `json:"actor"`
	ActorName *string                 `json:"actor_name"`
	From      map[string]any          `json:"from"`
	To        map[string]any          `json:"to"`
	CreatedAt time.Time               `json:"created_at"`
}

type EnquiryDTO struct {
	Ref               string               `json:"ref"`
	Channel           domain.Channel       `json:"channel"`
	SenderName        *string              `json:"sender_name"`
	SenderEmail       *string              `json:"sender_email"`
	SenderPhone       *string              `json:"sender_phone"`
	Subject           *string              `json:"subject"`
	Text              string               `json:"text"`
	ReceivedAt        time.Time            `json:"received_at"`
	Language          *string              `json:"language"`
	Category          *domain.Category     `json:"category"`
	SecondaryCategory *domain.Category     `json:"secondary_category"`
	Confidence        *float64             `json:"confidence"`
	AIReason          string               `json:"ai_reason"`
	Summary           *string              `json:"summary"`
	Extracted         ExtractedDTO         `json:"extracted"`
	Lane              domain.Lane          `json:"lane"`
	AssignedTo        *StaffRefDTO         `json:"assigned_to"`
	Priority          domain.Priority      `json:"priority"`
	Status            domain.EnquiryStatus `json:"status"`
	AIStatus          domain.AIStatus      `json:"ai_status"`
	AIModel           *string              `json:"ai_model"`
	FirstReplyAt      *time.Time           `json:"first_reply_at"`
	Draft             *DraftDTO            `json:"draft"`
	FollowUps         []FollowUpDTO        `json:"follow_ups"`
	Events            []EnquiryEventDTO    `json:"events"`
}

type EnquiryResponse struct {
	Enquiry EnquiryDTO `json:"enquiry"`
}

type RouteEnquiryRequest struct {
	Lane     string `json:"lane"`
	Category string `json:"category"`
}

type CorrectEnquiryRequest struct {
	CheckIn    *string `json:"check_in"`
	CheckOut   *string `json:"check_out"`
	Adults     *int    `json:"adults"`
	Children   *int    `json:"children"`
	Rooms      *int    `json:"rooms"`
	GuestCount *int    `json:"guest_count"`
}

type EditDraftRequest struct {
	Body string `json:"body"`
}

type EnquiryStatusRequest struct {
	Status string `json:"status"`
}

type DashboardResponse struct {
	AIEnabled               bool                `json:"ai_enabled"`
	MedianFirstReplyMinutes *float64            `json:"median_first_reply_minutes"`
	FirstReplyTargetMinutes int                 `json:"first_reply_target_minutes"`
	RepliedThisWeek         int                 `json:"replied_this_week"`
	OpenByLane              map[domain.Lane]int `json:"open_by_lane"`
	OpenComplaints          int                 `json:"open_complaints"`
	AwaitingAI              int                 `json:"awaiting_ai"`
	AICorrectionRate        *float64            `json:"ai_correction_rate"`
	DirectBookingsThisWeek  int                 `json:"direct_bookings_this_week"`
	DirectRevenueThisWeek   int64               `json:"direct_revenue_this_week_paise"`
	Overbookings            int                 `json:"overbookings"`
	Today                   string              `json:"today"`
	Overdue                 []WaitingDTO        `json:"overdue"`
	OverdueCount            int                 `json:"overdue_count"`
	Stays                   []DeskStayDTO       `json:"stays"`
	SoldTonight             int                 `json:"sold_tonight"`
	AllotmentTonight        int                 `json:"allotment_tonight"`
	RefundsPending          int                 `json:"refunds_pending"`
}

type WaitingDTO struct {
	Ref        string           `json:"ref"`
	Lane       *domain.Lane     `json:"lane"`
	Category   *domain.Category `json:"category"`
	Priority   domain.Priority  `json:"priority"`
	ReceivedAt time.Time        `json:"received_at"`
	Snippet    string           `json:"snippet"`
}

type DeskStayDTO struct {
	Ref       string `json:"ref"`
	GuestName string `json:"guest_name"`
	RoomType  string `json:"room_type"`
	CheckIn   string `json:"check_in"`
	CheckOut  string `json:"check_out"`
	Rooms     int    `json:"rooms"`
	Adults    int    `json:"adults"`
	Children  int    `json:"children"`
}

// firstReplyTargetMinutes is the brief's success measure for enquiries.
var firstReplyTargetMinutes = int(enquiry.FirstReplyTarget.Minutes())

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
		HoldExpiresAt: b.HoldExpiresAt, CreatedAt: b.CreatedAt, ConfirmedAt: b.ConfirmedAt, ArchivedAt: b.ArchivedAt}
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

func ratesGridDTO(g admin.Grid) RatesGridResponse {
	out := RatesGridResponse{Plan: g.Plan, Stay: stayDTO(g.Stay), Rows: make([]RateRowDTO, len(g.Rows))}
	for i, r := range g.Rows {
		row := RateRowDTO{RoomTypeID: r.RoomTypeID, Slug: r.Slug, Name: r.Name, TotalKeys: r.TotalKeys, Nights: make([]RateNightDTO, len(r.Nights))}
		for j, n := range r.Nights {
			row.Nights[j] = RateNightDTO{Date: date(n.Date), PricePaise: n.PricePaise, Allotment: n.Allotment, Sold: n.Sold,
				Held: n.Held, Left: n.Left, StopSell: n.StopSell, StopSellReason: n.StopSellReason, MinStay: n.MinStay, NoArrival: n.NoArrival}
		}
		out.Rows[i] = row
	}
	return out
}

func enquirySummaryDTO(e enquiry.Summary) EnquirySummaryDTO {
	return EnquirySummaryDTO{Ref: e.Ref, Channel: e.Channel, SenderName: e.SenderName, Snippet: e.Snippet, ReceivedAt: e.ReceivedAt,
		Category: e.Category, Confidence: e.Confidence, Lane: e.Lane, Status: e.Status, Priority: e.Priority, AIStatus: e.AIStatus,
		NeedsCheck: e.NeedsCheck, HasDraft: e.HasDraft, FollowUps: e.FollowUps}
}

func enquiryDTO(e enquiry.Enquiry) EnquiryDTO {
	out := EnquiryDTO{Ref: e.Ref, Channel: e.Channel, SenderName: e.SenderName, SenderEmail: e.SenderEmail, SenderPhone: e.SenderPhone,
		Subject: e.Subject, Text: e.Text, ReceivedAt: e.ReceivedAt, Language: e.Language, Category: e.Category,
		SecondaryCategory: e.Secondary, Confidence: e.Confidence, AIReason: e.AIReason, Summary: e.Summary, Lane: e.Lane,
		Priority: e.Priority, Status: e.Status, AIStatus: e.AIStatus, AIModel: e.AIModel, FirstReplyAt: e.FirstReply,
		Events: make([]EnquiryEventDTO, len(e.Events))}
	x := e.Extracted
	day := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := date(*t)
		return &s
	}
	out.Extracted = ExtractedDTO{CheckIn: day(x.CheckIn), CheckOut: day(x.CheckOut), Adults: x.Adults, Children: x.Children,
		Rooms: x.Rooms, GuestCount: x.GuestCount, EventMonth: x.EventMonth}
	if x.CheckIn != nil && x.CheckOut != nil {
		n := domain.NightsBetween(*x.CheckIn, *x.CheckOut)
		out.Extracted.Nights = &n
	}
	if x.BudgetINR != nil {
		p := *x.BudgetINR * 100
		out.Extracted.BudgetPaise = &p
	}
	if x.BudgetBasis != nil {
		b := string(*x.BudgetBasis)
		out.Extracted.BudgetBasis = &b
	}
	if e.AssignedTo != nil {
		out.AssignedTo = &StaffRefDTO{ID: e.AssignedTo.ID, Name: e.AssignedTo.Name, Role: e.AssignedTo.Role}
	}
	if d := e.Draft; d != nil {
		out.Draft = &DraftDTO{Version: d.Version, Body: d.Body, Author: d.Author, FlaggedReason: d.FlaggedReason,
			QuoteFilled: d.QuoteFilled, CreatedAt: d.CreatedAt, SentAt: d.SentAt, SignOff: d.SignOff}
	}
	out.FollowUps = make([]FollowUpDTO, len(e.FollowUps))
	for i, f := range e.FollowUps {
		out.FollowUps[i] = FollowUpDTO{Text: f.Text, ReceivedAt: f.ReceivedAt}
	}
	for i, ev := range e.Events {
		out.Events[i] = EnquiryEventDTO{Type: ev.Type, Actor: ev.Actor, ActorName: ev.ActorName, From: ev.From, To: ev.To, CreatedAt: ev.CreatedAt}
	}
	return out
}

func dashboardDTO(s enquiry.Stats, d booking.Desk, aiEnabled bool) DashboardResponse {
	r := DashboardResponse{AIEnabled: aiEnabled, MedianFirstReplyMinutes: s.MedianFirstReplyMinutes,
		FirstReplyTargetMinutes: firstReplyTargetMinutes, RepliedThisWeek: s.RepliedThisWeek, OpenByLane: s.OpenByLane,
		OpenComplaints: s.OpenComplaints, AwaitingAI: s.AwaitingAI, AICorrectionRate: s.AICorrectionRate,
		DirectBookingsThisWeek: s.DirectBookingsThisWeek, DirectRevenueThisWeek: s.DirectRevenueThisWeek, Overbookings: s.Overbookings,
		Today: d.Today.Format(time.DateOnly), Overdue: make([]WaitingDTO, 0, len(s.Overdue)), OverdueCount: s.OverdueCount,
		Stays: make([]DeskStayDTO, 0, len(d.Stays)), SoldTonight: d.SoldTonight, AllotmentTonight: d.AllotmentTonight,
		RefundsPending: d.RefundsPending}
	for _, w := range s.Overdue {
		r.Overdue = append(r.Overdue, WaitingDTO{Ref: w.Ref, Lane: w.Lane, Category: w.Category, Priority: w.Priority,
			ReceivedAt: w.ReceivedAt, Snippet: w.Snippet})
	}
	for _, st := range d.Stays {
		r.Stays = append(r.Stays, DeskStayDTO{Ref: st.Ref, GuestName: st.GuestName, RoomType: st.RoomType,
			CheckIn: st.CheckIn.Format(time.DateOnly), CheckOut: st.CheckOut.Format(time.DateOnly),
			Rooms: st.Rooms, Adults: st.Adults, Children: st.Children})
	}
	return r
}

// RetentionResponse reports one retention sweep.
type RetentionResponse struct {
	Result retention.Result `json:"result"`
}

// ---- analytics

type AnalyticsResponse struct {
	Range      AnalyticsRangeDTO       `json:"range"`
	KPIs       AnalyticsKPIsDTO        `json:"kpis"`
	Daily      []AnalyticsDayDTO       `json:"daily"`
	ByRoom     []AnalyticsBreakdownDTO `json:"by_room"`
	ByPlan     []AnalyticsBreakdownDTO `json:"by_plan"`
	ByCurrency []AnalyticsBreakdownDTO `json:"by_display_currency"`
	StatusMix  []AnalyticsCountDTO     `json:"status_mix"`
	LeadTime   []AnalyticsCountDTO     `json:"lead_time"`
	OnTheBooks []AnalyticsMonthDTO     `json:"on_the_books"`
	Enquiries  AnalyticsEnquiriesDTO   `json:"enquiries"`
}

type AnalyticsRangeDTO struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AnalyticsKPIsDTO struct {
	RevenuePaise         int64    `json:"revenue_paise"`
	RoomRevenuePaise     int64    `json:"room_revenue_paise"`
	GSTPaise             int64    `json:"gst_paise"`
	Bookings             int      `json:"bookings"`
	RoomNights           int      `json:"room_nights"`
	ADRPaise             int64    `json:"adr_paise"`
	AvgStayNights        float64  `json:"avg_stay_nights"`
	AvgLeadDays          float64  `json:"avg_lead_days"`
	CancellationRate     *float64 `json:"cancellation_rate"`
	CommissionSavedPaise int64    `json:"commission_saved_paise"`
	AvgBookingValuePaise int64    `json:"avg_booking_value_paise"`
	PreviousRevenuePaise int64    `json:"previous_revenue_paise"`
	PreviousBookings     int      `json:"previous_bookings"`
}

type AnalyticsDayDTO struct {
	Date         string `json:"date"`
	Bookings     int    `json:"bookings"`
	RevenuePaise int64  `json:"revenue_paise"`
}

type AnalyticsBreakdownDTO struct {
	Name         string `json:"name"`
	Bookings     int    `json:"bookings"`
	RoomNights   int    `json:"room_nights"`
	RevenuePaise int64  `json:"revenue_paise"`
}

type AnalyticsCountDTO struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type AnalyticsMonthDTO struct {
	Month            string `json:"month"`
	RoomNights       int    `json:"room_nights"`
	RoomRevenuePaise int64  `json:"room_revenue_paise"`
	Allotment        int    `json:"allotment"`
	Sold             int    `json:"sold"`
}

type AnalyticsEnquiriesDTO struct {
	Total                   int                 `json:"total"`
	ByChannel               []AnalyticsCountDTO `json:"by_channel"`
	ByCategory              []AnalyticsCountDTO `json:"by_category"`
	Converted               int                 `json:"converted"`
	MedianFirstReplyMinutes *float64            `json:"median_first_reply_minutes"`
	AIHandledRate           *float64            `json:"ai_handled_rate"`
}

// ---- gmail and composing

type GmailStatusDTO struct {
	Configured   bool       `json:"configured"`
	Connected    bool       `json:"connected"`
	AccountEmail *string    `json:"account_email"`
	ConnectedAt  *time.Time `json:"connected_at"`
	LastSyncAt   *time.Time `json:"last_sync_at"`
	LastError    *string    `json:"last_error"`
}

type GmailStatusResponse struct {
	Gmail GmailStatusDTO `json:"gmail"`
}

type GmailConnectResponse struct {
	AuthURL string `json:"auth_url"`
}

type GmailSyncResponse struct {
	Imported int            `json:"imported"`
	Gmail    GmailStatusDTO `json:"gmail"`
}

type ComposeRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type OutboxMessageResponse struct {
	Message OutboxMessageDTO `json:"message"`
}

type FxRateDTO struct {
	Currency         domain.Currency `json:"currency"`
	INRPerUnitMicros int64           `json:"inr_per_unit_micros"`
	Date             string          `json:"date"`
}

type FxRatesResponse struct {
	ChargedIn domain.Currency `json:"charged_in"`
	Rates     []FxRateDTO     `json:"rates"`
}

// ---- team and access

type PermissionDTO struct {
	Key         auth.Permission `json:"key"`
	Group       string          `json:"group"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
}

type RoleDTO struct {
	ID          uuid.UUID         `json:"id"`
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Team        domain.Role       `json:"team"`
	System      bool              `json:"system"`
	Permissions []auth.Permission `json:"permissions"`
	Members     int               `json:"members"`
}

type AccessResponse struct {
	Permissions []PermissionDTO `json:"permissions"`
	Roles       []RoleDTO       `json:"roles"`
}

type RoleRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Team        string            `json:"team"`
	Permissions []auth.Permission `json:"permissions"`
}

type RoleResponse struct {
	Role RoleDTO `json:"role"`
}

type MemberDTO struct {
	ID        uuid.UUID   `json:"id"`
	Email     string      `json:"email"`
	Name      string      `json:"name"`
	Phone     *string     `json:"phone"`
	RoleID    uuid.UUID   `json:"role_id"`
	RoleName  string      `json:"role_name"`
	Team      domain.Role `json:"team"`
	Active    bool        `json:"active"`
	CreatedAt time.Time   `json:"created_at"`
}

type MembersResponse struct {
	Members []MemberDTO `json:"members"`
}

type MemberResponse struct {
	Member MemberDTO `json:"member"`
}

type CreateMemberRequest struct {
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Phone    string    `json:"phone"`
	RoleID   uuid.UUID `json:"role_id"`
	Password string    `json:"password"`
}

type UpdateMemberRequest struct {
	Name     *string    `json:"name"`
	Phone    *string    `json:"phone"`
	RoleID   *uuid.UUID `json:"role_id"`
	Active   *bool      `json:"active"`
	Password *string    `json:"password"`
}

// ---- offers

type OfferRequest struct {
	Code        string           `json:"code"`
	Description string           `json:"description"`
	Type        domain.PromoType `json:"type"`
	Value       int64            `json:"value"`
	StayFrom    string           `json:"stay_from"`
	StayTo      string           `json:"stay_to"`
	BookFrom    string           `json:"book_from"`
	BookTo      string           `json:"book_to"`
	MinNights   int              `json:"min_nights"`
	MaxUses     *int             `json:"max_uses"`
	RoomTypeIDs []uuid.UUID      `json:"room_type_ids"`
	AutoApply   bool             `json:"auto_apply"`
	Active      bool             `json:"active"`
	Announce    bool             `json:"announce"` // email it to guests who opted in to offers
}

type OfferDTO struct {
	OfferRequest
	ID        uuid.UUID `json:"id"`
	UsedCount int       `json:"used_count"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OffersResponse struct {
	Offers []OfferDTO `json:"offers"`
}

type OfferResponse struct {
	Offer OfferDTO `json:"offer"`
}

// ---- inbox counts (the sidebar)

type LaneCountDTO struct {
	Lane  domain.Lane `json:"lane"`
	Count int         `json:"count"`
}

type ReplyTimeDTO struct {
	MedianMinutes *float64 `json:"median_minutes"`
	TargetMinutes int      `json:"target_minutes"`
}

type InboxCountsResponse struct {
	Lanes     []LaneCountDTO `json:"lanes"`
	ReplyTime *ReplyTimeDTO  `json:"reply_time"` // null when the caller can't see the dashboard
}

// ---- campaigns

type CampaignDTO struct {
	ID         uuid.UUID  `json:"id"`
	Kind       string     `json:"kind"`
	Subject    string     `json:"subject"`
	Body       string     `json:"body"`
	Status     string     `json:"status"`
	SendAt     time.Time  `json:"send_at"`
	SentAt     *time.Time `json:"sent_at"`
	Recipients int        `json:"recipients"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CampaignsResponse struct {
	Campaigns   []CampaignDTO `json:"campaigns"`
	Subscribers int           `json:"subscribers"`
}

type CampaignRequest struct {
	Kind    string     `json:"kind"`
	Subject string     `json:"subject"`
	Body    string     `json:"body"`
	SendAt  *time.Time `json:"send_at"`
}

type CampaignResponse struct {
	Campaign CampaignDTO `json:"campaign"`
}

type UnsubscribeRequest struct {
	Token string `json:"token"`
}
