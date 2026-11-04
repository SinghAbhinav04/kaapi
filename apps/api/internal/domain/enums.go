// Package domain holds the closed sets of values used across the API. Each type mirrors a
// CHECK constraint in migrations/0001_init.sql, so Go and the database agree on every state.
package domain

import "slices"

// Enum is satisfied by every string enum in this package.
type Enum interface{ ~string }

func valid[T Enum](v T, all []T) bool { return slices.Contains(all, v) }

type Role string

const (
	RoleGuest        Role = "GUEST"
	RoleReservations Role = "RESERVATIONS"
	RoleSales        Role = "SALES"
	RoleGM           Role = "GM"
	RoleAdmin        Role = "ADMIN"
)

var Roles = []Role{RoleGuest, RoleReservations, RoleSales, RoleGM, RoleAdmin}

func (r Role) Valid() bool   { return valid(r, Roles) }
func (r Role) IsStaff() bool { return r.Valid() && r != RoleGuest }

type RoomKind string

const (
	RoomKindRoom  RoomKind = "ROOM"
	RoomKindVilla RoomKind = "VILLA"
)

type CancellationPolicy string

const (
	CancelFreeUntil7Days CancellationPolicy = "FREE_UNTIL_7_DAYS"
	CancelNonRefundable  CancellationPolicy = "NON_REFUNDABLE"
)

type HoldStatus string

const (
	HoldActive    HoldStatus = "ACTIVE"
	HoldConverted HoldStatus = "CONVERTED"
	HoldReleased  HoldStatus = "RELEASED"
	HoldExpired   HoldStatus = "EXPIRED"
)

var HoldStatuses = []HoldStatus{HoldActive, HoldConverted, HoldReleased, HoldExpired}

func (s HoldStatus) Valid() bool { return valid(s, HoldStatuses) }

type BookingStatus string

const (
	BookingPendingPayment BookingStatus = "PENDING_PAYMENT"
	BookingConfirmed      BookingStatus = "CONFIRMED"
	BookingPaymentFailed  BookingStatus = "PAYMENT_FAILED"
	BookingExpired        BookingStatus = "EXPIRED"
	BookingCancelled      BookingStatus = "CANCELLED"
	BookingRefundPending  BookingStatus = "REFUND_PENDING"
	BookingRefunded       BookingStatus = "REFUNDED"
)

var BookingStatuses = []BookingStatus{BookingPendingPayment, BookingConfirmed, BookingPaymentFailed,
	BookingExpired, BookingCancelled, BookingRefundPending, BookingRefunded}

func (s BookingStatus) Valid() bool { return valid(s, BookingStatuses) }

// HoldsInventory reports whether a booking in this state occupies rooms.
func (s BookingStatus) HoldsInventory() bool { return s == BookingConfirmed }

type PaymentStatus string

const (
	PaymentCreated       PaymentStatus = "CREATED"
	PaymentCaptured      PaymentStatus = "CAPTURED"
	PaymentFailed        PaymentStatus = "FAILED"
	PaymentRefundPending PaymentStatus = "REFUND_PENDING"
	PaymentRefunded      PaymentStatus = "REFUNDED"
)

var PaymentStatuses = []PaymentStatus{PaymentCreated, PaymentCaptured, PaymentFailed, PaymentRefundPending, PaymentRefunded}

func (s PaymentStatus) Valid() bool { return valid(s, PaymentStatuses) }

type PaymentProvider string

const PaymentProviderMock PaymentProvider = "MOCK"

type PromoType string

const (
	PromoPercent PromoType = "PERCENT"
	PromoFlat    PromoType = "FLAT"
)

type Channel string

const (
	ChannelWebForm   Channel = "WEB_FORM"
	ChannelEmail     Channel = "EMAIL"
	ChannelWhatsApp  Channel = "WHATSAPP"
	ChannelEventForm Channel = "EVENT_FORM"
)

var Channels = []Channel{ChannelWebForm, ChannelEmail, ChannelWhatsApp, ChannelEventForm}

func (c Channel) Valid() bool { return valid(c, Channels) }

type Category string

const (
	CategoryRoom      Category = "ROOM"
	CategoryWedding   Category = "WEDDING"
	CategoryOffsite   Category = "OFFSITE"
	CategoryComplaint Category = "COMPLAINT"
	CategoryOther     Category = "OTHER"
)

var Categories = []Category{CategoryRoom, CategoryWedding, CategoryOffsite, CategoryComplaint, CategoryOther}

func (c Category) Valid() bool { return valid(c, Categories) }

type Lane string

const (
	LaneReservations Lane = "RESERVATIONS"
	LaneSales        Lane = "SALES"
	LaneGM           Lane = "GM"
	LaneUnsorted     Lane = "UNSORTED"
)

var Lanes = []Lane{LaneReservations, LaneSales, LaneGM, LaneUnsorted}

func (l Lane) Valid() bool { return valid(l, Lanes) }

type EnquiryStatus string

const (
	EnquiryNew        EnquiryStatus = "NEW"
	EnquiryInProgress EnquiryStatus = "IN_PROGRESS"
	EnquiryReplied    EnquiryStatus = "REPLIED"
	EnquiryClosed     EnquiryStatus = "CLOSED"
	EnquirySpam       EnquiryStatus = "SPAM"
)

var EnquiryStatuses = []EnquiryStatus{EnquiryNew, EnquiryInProgress, EnquiryReplied, EnquiryClosed, EnquirySpam}

func (s EnquiryStatus) Valid() bool { return valid(s, EnquiryStatuses) }

type Priority string

const (
	PriorityNormal Priority = "NORMAL"
	PriorityHigh   Priority = "HIGH"
)

type RoutedBy string

const (
	RoutedByAI    RoutedBy = "AI"
	RoutedByRule  RoutedBy = "RULE"
	RoutedByHuman RoutedBy = "HUMAN"
)

type AIStatus string

const (
	AIPending  AIStatus = "PENDING"
	AIDone     AIStatus = "DONE"
	AIFallback AIStatus = "FALLBACK"
	AIFailed   AIStatus = "FAILED"
)

type EnquiryEventType string

const (
	EventReceived        EnquiryEventType = "RECEIVED"
	EventClassified      EnquiryEventType = "CLASSIFIED"
	EventRerouted        EnquiryEventType = "REROUTED"
	EventCategoryChanged EnquiryEventType = "CATEGORY_CHANGED"
	EventFieldsCorrected EnquiryEventType = "FIELDS_CORRECTED"
	EventAssigned        EnquiryEventType = "ASSIGNED"
	EventDraftCreated    EnquiryEventType = "DRAFT_CREATED"
	EventDraftEdited     EnquiryEventType = "DRAFT_EDITED"
	EventSent            EnquiryEventType = "SENT"
	EventAlerted         EnquiryEventType = "ALERTED"
	EventClosed          EnquiryEventType = "CLOSED"
)

type Actor string

const (
	ActorAI     Actor = "AI"
	ActorSystem Actor = "SYSTEM"
	ActorUser   Actor = "USER"
)

type OutboxChannel string

const (
	OutboxEmail    OutboxChannel = "EMAIL"
	OutboxWhatsApp OutboxChannel = "WHATSAPP"
	OutboxSMS      OutboxChannel = "SMS"
)

type OutboxKind string

const (
	OutboxOTP                 OutboxKind = "OTP"
	OutboxWelcome             OutboxKind = "WELCOME" // a guest's first sign-in created their account
	OutboxBookingConfirmation OutboxKind = "BOOKING_CONFIRMATION"
	OutboxEnquiryReply        OutboxKind = "ENQUIRY_REPLY"
	OutboxAlert               OutboxKind = "ALERT"
	OutboxRefundNotice        OutboxKind = "REFUND_NOTICE"
	OutboxStaffMessage        OutboxKind = "STAFF_MESSAGE" // written from scratch by staff, not a reply
	OutboxStaffCode           OutboxKind = "STAFF_CODE"    // the second step of staff sign-in
	OutboxCheckInReminder     OutboxKind = "CHECKIN_REMINDER"
	OutboxCheckOutReminder    OutboxKind = "CHECKOUT_REMINDER"
)

type OutboxStatus string

const (
	OutboxQueued    OutboxStatus = "QUEUED"    // waiting for the worker to deliver it
	OutboxSimulated OutboxStatus = "SIMULATED" // recorded only: no mailbox connected
	OutboxSent      OutboxStatus = "SENT"      // delivered through the connected mailbox
	OutboxFailed    OutboxStatus = "FAILED"
)

type NotificationType string

const (
	NotifyComplaintAlert NotificationType = "COMPLAINT_ALERT"
	NotifyUnsorted       NotificationType = "UNSORTED"
	NotifyBooking        NotificationType = "BOOKING"
	NotifyRefundNeeded   NotificationType = "REFUND_NEEDED"
)

type OTPPurpose string

const OTPGuestLogin OTPPurpose = "GUEST_LOGIN"

type BookingSource string

const BookingSourceDirectWeb BookingSource = "DIRECT_WEB"

// Currency is one a guest can see prices in. Every booking is charged in INR; the others are
// an estimate for reading the price, from the ECB's daily reference rates (which have no AED).
type Currency string

const (
	CurrencyINR Currency = "INR"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
	CurrencyAUD Currency = "AUD"
	CurrencyCAD Currency = "CAD"
	CurrencySGD Currency = "SGD"
	CurrencyCHF Currency = "CHF"
	CurrencyJPY Currency = "JPY"
)

var Currencies = []Currency{CurrencyINR, CurrencyUSD, CurrencyEUR, CurrencyGBP, CurrencyAUD, CurrencyCAD, CurrencySGD, CurrencyCHF, CurrencyJPY}

func (c Currency) Valid() bool { return valid(c, Currencies) }
