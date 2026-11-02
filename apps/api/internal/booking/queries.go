package booking

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/money"
)

// bookingSelect reads a booking with its room, plan, hold expiry and latest payment.
const bookingSelect = `SELECT b.id, b.ref, b.status, b.room_type_id, rt.name, rt.slug, rp.name, rp.refundable,
	b.check_in, b.check_out, b.rooms, b.adults, b.children,
	b.guest_name, b.guest_email, b.guest_phone, b.guest_country, b.arrival_window, b.special_requests,
	b.price_breakdown, h.expires_at, b.created_at, b.confirmed_at,
	p.provider_order_id, p.status, p.amount_paise, p.failure_reason
	FROM bookings b
	JOIN room_types rt ON rt.id = b.room_type_id
	JOIN rate_plans rp ON rp.id = b.rate_plan_id
	LEFT JOIN holds h ON h.id = b.hold_id
	LEFT JOIN LATERAL (SELECT * FROM payments WHERE booking_id = b.id ORDER BY created_at DESC LIMIT 1) p ON true`

func scanBooking(r pgx.CollectableRow) (Booking, error) {
	var b Booking
	var quote []byte
	var orderID *string
	var payStatus *domain.PaymentStatus
	var amount *int64
	var failure *string
	err := r.Scan(&b.ID, &b.Ref, &b.Status, &b.RoomTypeID, &b.RoomTypeName, &b.RoomTypeSlug, &b.RatePlanName, &b.Refundable,
		&b.CheckIn, &b.CheckOut, &b.Rooms, &b.Adults, &b.Children,
		&b.Guest.Name, &b.Guest.Email, &b.Guest.Phone, &b.Guest.Country, &b.ArrivalWindow, &b.SpecialRequests,
		&quote, &b.HoldExpiresAt, &b.CreatedAt, &b.ConfirmedAt, &orderID, &payStatus, &amount, &failure)
	if err != nil {
		return b, err
	}
	if err := json.Unmarshal(quote, &b.Quote); err != nil {
		return b, fmt.Errorf("booking %s quote: %w", b.Ref, err)
	}
	if orderID != nil {
		b.Payment = &Payment{OrderID: *orderID, Status: *payStatus, AmountPaise: *amount, FailureReason: failure}
	}
	return b, nil
}

func (s *Service) one(ctx context.Context, where string, arg any) (Booking, error) {
	rows, err := s.pool.Query(ctx, bookingSelect+" WHERE "+where, arg)
	return collectOne(rows, err)
}

func (s *Service) oneTx(ctx context.Context, tx pgx.Tx, where string, arg any) (Booking, error) {
	rows, err := tx.Query(ctx, bookingSelect+" WHERE "+where, arg)
	return collectOne(rows, err)
}

func collectOne(rows pgx.Rows, err error) (Booking, error) {
	if err != nil {
		return Booking{}, err
	}
	b, err := pgx.CollectExactlyOneRow(rows, scanBooking)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	return b, err
}

// ByRef finds a booking by its reference. Callers decide who may see it.
func (s *Service) ByRef(ctx context.Context, ref string) (Booking, error) {
	return s.one(ctx, `b.ref = $1`, strings.ToUpper(strings.TrimSpace(ref)))
}

// ByOrder finds the booking a payment order belongs to.
func (s *Service) ByOrder(ctx context.Context, orderID string) (Booking, error) {
	b, err := s.one(ctx, `p.provider_order_id = $1`, orderID)
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
	var stored []byte
	if err := s.pool.QueryRow(ctx, `SELECT h.hold_token_hash FROM bookings b JOIN holds h ON h.id = b.hold_id WHERE b.id = $1`,
		bookingID).Scan(&stored); err != nil {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(sum[:], stored) == 1
}

// Mine returns a guest's bookings: made while signed in, or under their email.
func (s *Service) Mine(ctx context.Context, userID uuid.UUID, email string) ([]Booking, error) {
	rows, err := s.pool.Query(ctx, bookingSelect+` WHERE b.guest_user_id = $1 OR lower(b.guest_email) = lower($2)
		ORDER BY b.check_in DESC`, userID, email)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanBooking)
}

// List returns bookings for staff, newest check-in first.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Booking, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("b.status = $%d", f.Status)
	}
	if !f.From.IsZero() {
		add("b.check_in >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("b.check_in < $%d", f.To)
	}
	q := bookingSelect
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, f.Limit)
	q += fmt.Sprintf(" ORDER BY b.check_in, b.created_at LIMIT $%d", len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanBooking)
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
	rows, err := s.pool.Query(ctx, bookingSelect+` WHERE (b.confirmed_at >= $1 AND b.confirmed_at < $2)
		OR (b.cancelled_at >= $1 AND b.cancelled_at < $2) ORDER BY b.confirmed_at`, start, end)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, scanBooking)
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
	_, err = s.pool.Exec(ctx, `UPDATE bookings SET exported_at = now() WHERE id = ANY($1)`, ids)
	return err
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
