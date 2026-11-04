package booking

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/platform/money"
)

// ByRef finds a booking by its reference. Callers decide who may see it.
func (s *Service) ByRef(ctx context.Context, ref string) (Booking, error) {
	return s.store.one(ctx, s.pool, `b.ref = $1`, strings.ToUpper(strings.TrimSpace(ref)))
}

// ByOrder finds the booking a payment order belongs to.
func (s *Service) ByOrder(ctx context.Context, orderID string) (Booking, error) {
	b, err := s.store.one(ctx, s.pool, `p.provider_order_id = $1`, orderID)
	if errors.Is(err, ErrBookingNotFound) {
		return b, ErrOrderNotFound
	}
	return b, err
}

// HoldTokenMatches lets the browser that made a booking see it without signing in.
func (s *Service) HoldTokenMatches(ctx context.Context, bookingID uuid.UUID, token string) bool {
	if token == "" {
		return false
	}
	stored, err := s.store.holdTokenHash(ctx, s.pool, bookingID)
	if err != nil {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(sum[:], stored) == 1
}

// Mine returns a guest's bookings: made while signed in, or under their email.
func (s *Service) Mine(ctx context.Context, userID uuid.UUID, email string) ([]Booking, error) {
	return s.store.mine(ctx, s.pool, userID, email)
}

// List returns bookings for staff, newest check-in first.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Booking, error) {
	return s.store.list(ctx, s.pool, f)
}

// exportColumns is the layout the resort's reservation system imports, one row per booking.
var exportColumns = []string{"booking_ref", "status", "guest_name", "guest_email", "guest_phone", "country",
	"room_type", "rate_plan", "check_in", "check_out", "nights", "rooms", "adults", "children",
	"room_total_inr", "gst_inr", "paid_inr", "arrival_window", "special_requests", "confirmed_at_ist"}

// ExportDay writes every booking confirmed or cancelled on a date (IST) as CSV and stamps
// them as exported, so the desk can key the day's direct bookings into their system.
func (s *Service) ExportDay(ctx context.Context, w io.Writer, day time.Time, loc *time.Location) error {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	list, err := s.store.settledBetween(ctx, s.pool, start, end)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(exportColumns); err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(list))
	for _, b := range list {
		confirmed := ""
		if b.ConfirmedAt != nil {
			confirmed = b.ConfirmedAt.In(loc).Format("2006-01-02 15:04")
		}
		rec := []string{b.Ref, string(b.Status), b.Guest.Name, b.Guest.Email, b.Guest.Phone, b.Guest.Country,
			b.RoomTypeName, b.RatePlanName, b.CheckIn.Format(time.DateOnly), b.CheckOut.Format(time.DateOnly),
			strconv.Itoa(b.Nights()), strconv.Itoa(b.Rooms), strconv.Itoa(b.Adults), strconv.Itoa(b.Children),
			rupees(b.Quote.SubtotalPaise - b.Quote.DiscountPaise), rupees(b.Quote.TaxPaise), rupees(b.Quote.TotalPaise),
			deref(b.ArrivalWindow), deref(b.SpecialRequests), confirmed}
		if err := cw.Write(rec); err != nil {
			return err
		}
		ids = append(ids, b.ID)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return s.store.markExported(ctx, s.pool, ids)
}

// rupees renders paise as a plain decimal for spreadsheets (no ₹ or grouping).
func rupees(paise int64) string {
	return strings.TrimPrefix(strings.ReplaceAll(money.FormatINR(paise), ",", ""), "₹")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
