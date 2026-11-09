package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/fx"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
	"kaapi/api/internal/pricing"
)

// store is every query this package runs. Guest details are encrypted at rest (package
// pii) and found by the email's blind index.
type store struct{}

// bookingSelect reads a booking with its room, plan, hold expiry and latest payment.
const bookingSelect = `SELECT b.id, b.ref, b.status, b.room_type_id, rt.name, rt.slug, rp.name, rp.refundable,
	b.check_in, b.check_out, b.rooms, b.adults, b.children,
	b.guest_name, b.guest_email, b.guest_phone, b.guest_country, b.arrival_window, b.special_requests,
	b.price_breakdown, h.expires_at, b.created_at, b.confirmed_at, b.archived_at,
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
	var name, email, phone, country pii.Text // decrypted on scan
	err := r.Scan(&b.ID, &b.Ref, &b.Status, &b.RoomTypeID, &b.RoomTypeName, &b.RoomTypeSlug, &b.RatePlanName, &b.Refundable,
		&b.CheckIn, &b.CheckOut, &b.Rooms, &b.Adults, &b.Children,
		&name, &email, &phone, &country, &b.ArrivalWindow, &b.SpecialRequests,
		&quote, &b.HoldExpiresAt, &b.CreatedAt, &b.ConfirmedAt, &b.ArchivedAt, &orderID, &payStatus, &amount, &failure)
	if err != nil {
		return b, err
	}
	b.Guest = Guest{Name: string(name), Email: string(email), Phone: string(phone), Country: string(country)}
	if err := json.Unmarshal(quote, &b.Quote); err != nil {
		return b, fmt.Errorf("booking %s quote: %w", b.Ref, err)
	}
	if orderID != nil {
		b.Payment = &Payment{OrderID: *orderID, Status: *payStatus, AmountPaise: *amount, FailureReason: failure}
	}
	return b, nil
}

// one returns ErrBookingNotFound when nothing matches. where is a constant in this package.
func (store) one(ctx context.Context, q db.Querier, where string, arg any) (Booking, error) {
	rows, err := q.Query(ctx, bookingSelect+" WHERE "+where, arg)
	if err != nil {
		return Booking{}, err
	}
	b, err := pgx.CollectExactlyOneRow(rows, scanBooking)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	return b, err
}

// deskStays lists confirmed stays that arrive or leave in [from, to).
func (store) deskStays(ctx context.Context, q db.Querier, from, to time.Time, limit int) ([]DeskStay, error) {
	rows, err := q.Query(ctx, `SELECT b.ref, b.guest_name, rt.name, b.check_in, b.check_out, b.rooms, b.adults, b.children
		FROM bookings b JOIN room_types rt ON rt.id = b.room_type_id
		WHERE b.status = $3 AND ((b.check_in >= $1 AND b.check_in < $2) OR (b.check_out >= $1 AND b.check_out < $2))
		ORDER BY b.check_in, b.ref LIMIT $4`, from, to, domain.BookingConfirmed, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DeskStay, error) {
		var d DeskStay
		return d, r.Scan(&d.Ref, pii.Into(&d.GuestName), &d.RoomType, &d.CheckIn, &d.CheckOut, &d.Rooms, &d.Adults, &d.Children)
	})
}

// deskCounts fills tonight's website occupancy and the refunds still owed.
func (store) deskCounts(ctx context.Context, q db.Querier, today time.Time, d *Desk) error {
	return q.QueryRow(ctx, `SELECT coalesce(sum(sold), 0)::int, coalesce(sum(allotment), 0)::int,
		(SELECT count(*) FROM bookings WHERE status = $2)::int
		FROM night_inventory WHERE date = $1`, today, domain.BookingRefundPending).
		Scan(&d.SoldTonight, &d.AllotmentTonight, &d.RefundsPending)
}

func (store) many(ctx context.Context, q db.Querier, sql string, args ...any) ([]Booking, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanBooking)
}

func (s store) mine(ctx context.Context, q db.Querier, userID uuid.UUID, email string) ([]Booking, error) {
	return s.many(ctx, q, bookingSelect+` WHERE b.guest_user_id = $1 OR b.guest_email_hash = $2 ORDER BY b.check_in DESC`,
		userID, pii.EmailIndex(email))
}

// list is the staff list: live bookings in stay order, or the archive newest first.
func (s store) list(ctx context.Context, q db.Querier, f ListFilter) ([]Booking, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("b.status = $%d", f.Status)
	}
	if f.Archived {
		where = append(where, "b.archived_at IS NOT NULL")
	} else {
		where = append(where, "b.archived_at IS NULL")
	}
	if !f.From.IsZero() {
		add("b.check_in >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("b.check_in < $%d", f.To)
	}
	args = append(args, f.Limit)
	order := "b.check_in, b.created_at"
	if f.Archived {
		order = "b.check_in DESC, b.created_at DESC"
	}
	return s.many(ctx, q, bookingSelect+" WHERE "+strings.Join(where, " AND ")+fmt.Sprintf(" ORDER BY %s LIMIT $%d", order, len(args)), args...)
}

// settledBetween is every booking confirmed or cancelled in [start, end).
func (s store) settledBetween(ctx context.Context, q db.Querier, start, end time.Time) ([]Booking, error) {
	return s.many(ctx, q, bookingSelect+` WHERE (b.confirmed_at >= $1 AND b.confirmed_at < $2)
		OR (b.cancelled_at >= $1 AND b.cancelled_at < $2) ORDER BY b.confirmed_at`, start, end)
}

func (store) markExported(ctx context.Context, q db.Querier, ids []uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE bookings SET exported_at = now() WHERE id = ANY($1)`, ids)
	return err
}

// holdTokenHash is the token hash of the hold a booking came from; pgx.ErrNoRows if none.
func (store) holdTokenHash(ctx context.Context, q db.Querier, bookingID uuid.UUID) ([]byte, error) {
	var h []byte
	err := q.QueryRow(ctx, `SELECT h.hold_token_hash FROM bookings b JOIN holds h ON h.id = b.hold_id WHERE b.id = $1`, bookingID).Scan(&h)
	return h, err
}

// ---- creating

// lockedHold is a hold as booking needs it, locked for the transaction.
type lockedHold struct {
	req       inventory.HoldRequest
	quoteJSON []byte
	promoID   *uuid.UUID
	status    domain.HoldStatus
	live      bool
	tokenHash []byte
}

// lockHold returns pgx.ErrNoRows for an unknown hold.
func (store) lockHold(ctx context.Context, q db.Querier, id uuid.UUID) (lockedHold, error) {
	var h lockedHold
	r := &h.req
	err := q.QueryRow(ctx, `SELECT room_type_id, rate_plan_id, check_in, check_out, rooms, adults, children, quote,
		promo_code_id, status, expires_at > now(), hold_token_hash FROM holds WHERE id = $1 FOR UPDATE`, id).
		Scan(&r.RoomTypeID, &r.RatePlanID, &r.CheckIn, &r.CheckOut, &r.Rooms, &r.Adults, &r.Children, &h.quoteJSON,
			&h.promoID, &h.status, &h.live, &h.tokenHash)
	return h, err
}

// bookingForHold returns pgx.ErrNoRows when the hold hasn't become a booking yet.
func (store) bookingForHold(ctx context.Context, q db.Querier, holdID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM bookings WHERE hold_id = $1`, holdID).Scan(&id)
	return id, err
}

// newBooking is everything inserted for a booking awaiting payment.
type newBooking struct {
	id, holdID uuid.UUID
	ref        string
	in         CreateInput
	hold       lockedHold
	quote      pricing.Quote
	at         time.Time
	shown      *domain.Currency
	rate       *fx.Rate
}

// insertBooking reports false when the ref was already taken (the caller tries another).
func (store) insertBooking(ctx context.Context, q db.Querier, n newBooking) (bool, error) {
	var rateMicros *int64
	var rateDate *time.Time
	if n.rate != nil {
		rateMicros, rateDate = &n.rate.INRPerUnitMicros, &n.rate.Date
	}
	h, in, qt := n.hold.req, n.in, n.quote
	tag, err := q.Exec(ctx, `INSERT INTO bookings
		(id, ref, hold_id, guest_user_id, guest_name, guest_email, guest_email_hash, guest_phone, guest_country, adults, children, rooms,
		 room_type_id, rate_plan_id, check_in, check_out, arrival_window, special_requests, status,
		 subtotal_paise, discount_paise, tax_paise, total_paise, price_breakdown, promo_code_id,
		 consent_version, consent_at, marketing_opt_in, source, display_currency, fx_inr_per_unit_micros, fx_rate_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)
		ON CONFLICT (ref) DO NOTHING`,
		n.id, n.ref, n.holdID, in.GuestUserID, pii.Text(in.Guest.Name), pii.Text(in.Guest.Email),
		pii.EmailIndex(in.Guest.Email), pii.Text(in.Guest.Phone), pii.Text(in.Guest.Country),
		h.Adults, h.Children, h.Rooms, h.RoomTypeID, h.RatePlanID, h.CheckIn, h.CheckOut, nilIfEmpty(in.ArrivalWindow),
		nilIfEmpty(in.SpecialRequests), domain.BookingPendingPayment, qt.SubtotalPaise, qt.DiscountPaise, qt.TaxPaise,
		qt.TotalPaise, n.hold.quoteJSON, n.hold.promoID, ConsentVersion, n.at, in.MarketingOptIn, domain.BookingSourceDirectWeb,
		n.shown, rateMicros, rateDate)
	return err == nil && tag.RowsAffected() == 1, err
}

func (store) insertPaymentOrder(ctx context.Context, q db.Querier, bookingID uuid.UUID, amount int64, idempotencyKey string) error {
	_, err := q.Exec(ctx, `INSERT INTO payments (id, booking_id, provider, provider_order_id, amount_paise, status, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, idgen.New(), bookingID, domain.PaymentProviderMock, payment.NewOrderID(),
		amount, domain.PaymentCreated, idempotencyKey)
	return err
}

// ---- payment events

// orderRow is a payment order with its booking, both locked.
type orderRow struct {
	paymentID, bookingID uuid.UUID
	status               domain.PaymentStatus
	amount               int64
	holdID               *uuid.UUID
	ref                  string
	guestEmail           pii.Text
	promoID              *uuid.UUID
}

// lockOrder returns pgx.ErrNoRows for an unknown order.
func (store) lockOrder(ctx context.Context, q db.Querier, orderID string) (orderRow, error) {
	var o orderRow
	err := q.QueryRow(ctx, `SELECT p.id, p.status, p.amount_paise, b.id, b.hold_id, b.ref, b.guest_email, b.promo_code_id
		FROM payments p JOIN bookings b ON b.id = p.booking_id WHERE p.provider_order_id = $1 FOR UPDATE OF p, b`, orderID).
		Scan(&o.paymentID, &o.status, &o.amount, &o.bookingID, &o.holdID, &o.ref, &o.guestEmail, &o.promoID)
	return o, err
}

// recordEvent reports false when this provider event was already recorded.
func (store) recordEvent(ctx context.Context, q db.Querier, e payment.Event, payload []byte) (bool, error) {
	tag, err := q.Exec(ctx, `INSERT INTO payment_events (id, provider, provider_event_id, type, payload, signature_valid, processed_at)
		VALUES ($1,$2,$3,$4,$5,true,now()) ON CONFLICT (provider, provider_event_id) DO NOTHING`,
		idgen.New(), e.Provider, e.ID, e.Type, payload)
	return err == nil && tag.RowsAffected() == 1, err
}

func (store) setPayment(ctx context.Context, q db.Querier, id uuid.UUID, status domain.PaymentStatus, providerPaymentID string, failure *string) error {
	_, err := q.Exec(ctx, `UPDATE payments SET status = $2, provider_payment_id = $3, failure_reason = $4, updated_at = now() WHERE id = $1`,
		id, status, providerPaymentID, failure)
	return err
}

func (store) setBookingStatus(ctx context.Context, q db.Querier, id uuid.UUID, status domain.BookingStatus) error {
	_, err := q.Exec(ctx, `UPDATE bookings SET status = $2, updated_at = now(),
		confirmed_at = CASE WHEN $2 = 'CONFIRMED' THEN now() ELSE confirmed_at END WHERE id = $1`, id, status)
	return err
}

// usePromo counts a use. The guest was shown this price, so it's honoured even if the last
// use went meanwhile (the count just stops at the limit).
func (store) usePromo(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE promo_codes SET used_count = used_count + 1 WHERE id = $1 AND (max_uses IS NULL OR used_count < max_uses)`, id)
	return err
}

func (store) notify(ctx context.Context, q db.Querier, role domain.Role, kind domain.NotificationType, bookingID uuid.UUID, body string) error {
	_, err := q.Exec(ctx, `INSERT INTO notifications (id, recipient_role, type, booking_id, body) VALUES ($1,$2,$3,$4,$5)`,
		idgen.New(), role, kind, bookingID, body)
	return err
}

// queueEmail puts a guest email in the outbox for the worker to send.
func (store) queueEmail(ctx context.Context, q db.Querier, kind domain.OutboxKind, to, subject, body string, bookingID uuid.UUID) error {
	_, err := q.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, status, recipient, subject, body, booking_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		idgen.New(), domain.OutboxEmail, kind, domain.OutboxQueued, pii.Text(to), subject, pii.Text(body), bookingID)
	return err
}

// ---- sweep

func (store) expireUnpaid(ctx context.Context, q db.Querier, grace time.Duration) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE bookings b SET status = $1, updated_at = now() FROM holds h
		WHERE h.id = b.hold_id AND b.status IN ($2, $3) AND h.expires_at < now() - make_interval(secs => $4)`,
		domain.BookingExpired, domain.BookingPendingPayment, domain.BookingPaymentFailed, grace.Seconds())
	return tag.RowsAffected(), err
}

func (store) expireHolds(ctx context.Context, q db.Querier, grace time.Duration) error {
	_, err := q.Exec(ctx, `UPDATE holds SET status = $1 WHERE status = $2 AND expires_at < now() - make_interval(secs => $3)`,
		domain.HoldExpired, domain.HoldActive, grace.Seconds())
	return err
}

// ---- reminders

// dueReminders locks confirmed, live bookings for which this reminder is due today and
// hasn't gone. today is the resort's calendar date.
func (store) dueReminders(ctx context.Context, q db.Querier, kind string, today time.Time, limit int) ([]uuid.UUID, error) {
	args := []any{domain.BookingConfirmed, today, kind, limit}
	var when string
	switch kind {
	case reminderCheckInSoon: // arriving in the next few days
		when, args = `b.check_in > $2 AND b.check_in <= $2::date + $5::int`, append(args, checkInSoonDays)
	case reminderCheckInToday:
		when = `b.check_in = $2`
	case reminderCheckOutSoon: // already staying, leaving in two days
		when, args = `b.check_out = $2::date + $5::int AND b.check_in < $2`, append(args, checkOutSoonDays)
	case reminderCheckOutToday:
		when = `b.check_out = $2`
	default:
		return nil, fmt.Errorf("booking: unknown reminder %q", kind)
	}
	rows, err := q.Query(ctx, `SELECT b.id FROM bookings b
		WHERE b.status = $1 AND b.archived_at IS NULL AND `+when+`
		  AND NOT EXISTS (SELECT 1 FROM booking_reminders r WHERE r.booking_id = b.id AND r.kind = $3)
		ORDER BY b.id LIMIT $4 FOR UPDATE OF b SKIP LOCKED`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// markReminded records a reminder; false if another run got there first.
func (store) markReminded(ctx context.Context, q db.Querier, bookingID uuid.UUID, kind string) (bool, error) {
	tag, err := q.Exec(ctx, `INSERT INTO booking_reminders (booking_id, kind) VALUES ($1, $2) ON CONFLICT DO NOTHING`, bookingID, kind)
	return err == nil && tag.RowsAffected() == 1, err
}
