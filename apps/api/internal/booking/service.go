// Package booking turns a hold into a booking, takes payment events and keeps the
// guest-facing record. Inventory itself is changed only through package inventory.
package booking

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/fx"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/money"
	"kaapi/api/internal/pricing"
)

var (
	ErrBookingNotFound = apperr.New(apperr.CodeNotFound, "We couldn't find that booking.")
	ErrOrderNotFound   = apperr.New(apperr.CodeNotFound, "We couldn't find that payment.")
	ErrAmountMismatch  = apperr.New(apperr.CodeConflict, "The paid amount doesn't match the booking.")
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, now: now}
}

// Create turns an active hold into a booking awaiting payment, with a payment order.
// Submitting twice for the same hold returns the same booking.
func (s *Service) Create(ctx context.Context, in CreateInput) (Booking, error) {
	if err := validate(&in); err != nil {
		return Booking{}, err
	}
	var bookingID uuid.UUID
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var (
			h         inventory.HoldRequest
			quoteJSON []byte
			promoID   *uuid.UUID
			status    domain.HoldStatus
			live      bool
			tokenHash []byte
		)
		err := tx.QueryRow(ctx, `SELECT room_type_id, rate_plan_id, check_in, check_out, rooms, adults, children, quote,
			promo_code_id, status, expires_at > now(), hold_token_hash FROM holds WHERE id = $1 FOR UPDATE`, in.HoldID).
			Scan(&h.RoomTypeID, &h.RatePlanID, &h.CheckIn, &h.CheckOut, &h.Rooms, &h.Adults, &h.Children, &quoteJSON,
				&promoID, &status, &live, &tokenHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return inventory.ErrHoldNotFound
		}
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(in.HoldToken))
		if subtle.ConstantTimeCompare(sum[:], tokenHash) != 1 {
			return inventory.ErrHoldNotFound
		}
		// Idempotent: a double-click or a retried request gets the booking already made.
		err = tx.QueryRow(ctx, `SELECT id FROM bookings WHERE hold_id = $1`, in.HoldID).Scan(&bookingID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if status != domain.HoldActive || !live {
			return inventory.ErrHoldNotActive
		}
		var q pricing.Quote
		if err := json.Unmarshal(quoteJSON, &q); err != nil {
			return fmt.Errorf("hold quote: %w", err)
		}

		// What the guest was reading prices in, with that moment's rate. Never part of the charge.
		var shown *domain.Currency
		var rate *fx.Rate
		if in.DisplayCurrency != "" {
			shown = &in.DisplayCurrency
			if rate, err = fx.Snapshot(ctx, tx, in.DisplayCurrency); err != nil {
				return err
			}
		}
		var rateMicros *int64
		var rateDate *time.Time
		if rate != nil {
			rateMicros, rateDate = &rate.INRPerUnitMicros, &rate.Date
		}

		now := s.now()
		for attempt := 0; attempt < refAttempts; attempt++ {
			id := idgen.New()
			tag, err := tx.Exec(ctx, `INSERT INTO bookings
				(id, ref, hold_id, guest_user_id, guest_name, guest_email, guest_phone, guest_country, adults, children, rooms,
				 room_type_id, rate_plan_id, check_in, check_out, arrival_window, special_requests, status,
				 subtotal_paise, discount_paise, tax_paise, total_paise, price_breakdown, promo_code_id,
				 consent_version, consent_at, marketing_opt_in, source, display_currency, fx_inr_per_unit_micros, fx_rate_date)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31)
				ON CONFLICT (ref) DO NOTHING`,
				id, idgen.BookingRef(), in.HoldID, in.GuestUserID, in.Guest.Name, in.Guest.Email, in.Guest.Phone, in.Guest.Country,
				h.Adults, h.Children, h.Rooms, h.RoomTypeID, h.RatePlanID, h.CheckIn, h.CheckOut, nilIfEmpty(in.ArrivalWindow),
				nilIfEmpty(in.SpecialRequests), domain.BookingPendingPayment, q.SubtotalPaise, q.DiscountPaise, q.TaxPaise,
				q.TotalPaise, quoteJSON, promoID, ConsentVersion, now, in.MarketingOptIn, domain.BookingSourceDirectWeb,
				shown, rateMicros, rateDate)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				bookingID = id
				break
			}
		}
		if bookingID == uuid.Nil {
			return errors.New("booking: could not allocate a unique reference")
		}
		_, err = tx.Exec(ctx, `INSERT INTO payments (id, booking_id, provider, provider_order_id, amount_paise, status, idempotency_key)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, idgen.New(), bookingID, domain.PaymentProviderMock, payment.NewOrderID(),
			q.TotalPaise, domain.PaymentCreated, in.HoldID.String())
		return err
	})
	if err != nil {
		return Booking{}, err
	}
	return s.one(ctx, `b.id = $1`, bookingID)
}

// HandlePaymentEvent applies a verified provider event. A repeated delivery (same event id)
// changes nothing. A capture that can no longer get its room takes the refund path.
func (s *Service) HandlePaymentEvent(ctx context.Context, e payment.Event) (Booking, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return Booking{}, err
	}
	var bookingID uuid.UUID
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var (
			paymentID, holdID uuid.UUID
			payStatus         domain.PaymentStatus
			amount            int64
			guestEmail, ref   string
			promoID           *uuid.UUID
			holdRef           *uuid.UUID
		)
		err := tx.QueryRow(ctx, `SELECT p.id, p.status, p.amount_paise, b.id, b.hold_id, b.ref, b.guest_email, b.promo_code_id
			FROM payments p JOIN bookings b ON b.id = p.booking_id WHERE p.provider_order_id = $1 FOR UPDATE OF p, b`, e.OrderID).
			Scan(&paymentID, &payStatus, &amount, &bookingID, &holdRef, &ref, &guestEmail, &promoID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO payment_events (id, provider, provider_event_id, type, payload, signature_valid, processed_at)
			VALUES ($1,$2,$3,$4,$5,true,now()) ON CONFLICT (provider, provider_event_id) DO NOTHING`,
			idgen.New(), e.Provider, e.ID, e.Type, payload)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 || payStatus == domain.PaymentCaptured {
			return nil // already processed, or already paid by an earlier event
		}
		if e.AmountPaise != amount {
			return ErrAmountMismatch
		}
		if holdRef != nil {
			holdID = *holdRef
		}

		switch e.Type {
		case payment.EventFailed:
			if _, err := tx.Exec(ctx, `UPDATE payments SET status = $2, provider_payment_id = $3, failure_reason = $4, updated_at = now() WHERE id = $1`,
				paymentID, domain.PaymentFailed, e.PaymentID, e.FailureReason); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE bookings SET status = $2, updated_at = now() WHERE id = $1`, bookingID, domain.BookingPaymentFailed)
			return err

		case payment.EventCaptured:
			err := inventory.Confirm(ctx, tx, holdID, bookingID)
			if errors.Is(err, inventory.ErrSoldOut) || errors.Is(err, inventory.ErrHoldNotActive) {
				return s.refundPath(ctx, tx, paymentID, bookingID, ref, guestEmail, e)
			}
			if err != nil {
				return err // rolled back; the provider retries the delivery
			}
			if _, err := tx.Exec(ctx, `UPDATE payments SET status = $2, provider_payment_id = $3, failure_reason = NULL, updated_at = now() WHERE id = $1`,
				paymentID, domain.PaymentCaptured, e.PaymentID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE bookings SET status = $2, confirmed_at = now(), updated_at = now() WHERE id = $1`,
				bookingID, domain.BookingConfirmed); err != nil {
				return err
			}
			if promoID != nil {
				// The guest was shown this price, so it's honoured even if the last use went meanwhile.
				if _, err := tx.Exec(ctx, `UPDATE promo_codes SET used_count = used_count + 1
					WHERE id = $1 AND (max_uses IS NULL OR used_count < max_uses)`, *promoID); err != nil {
					return err
				}
			}
			return s.confirmationMessages(ctx, tx, bookingID)
		}
		return apperr.New(apperr.CodeBadRequest, "Unknown payment event type.")
	})
	if err != nil {
		return Booking{}, err
	}
	return s.one(ctx, `b.id = $1`, bookingID)
}

func (s *Service) refundPath(ctx context.Context, tx pgx.Tx, paymentID, bookingID uuid.UUID, ref, email string, e payment.Event) error {
	if _, err := tx.Exec(ctx, `UPDATE payments SET status = $2, provider_payment_id = $3, updated_at = now() WHERE id = $1`,
		paymentID, domain.PaymentRefundPending, e.PaymentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE bookings SET status = $2, updated_at = now() WHERE id = $1`, bookingID, domain.BookingRefundPending); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notifications (id, recipient_role, type, booking_id, body) VALUES ($1,$2,$3,$4,$5)`,
		idgen.New(), domain.RoleGM, domain.NotifyRefundNeeded, bookingID,
		fmt.Sprintf("Refund %s for %s: the payment arrived after the room was taken.", money.FormatINR(e.AmountPaise), ref)); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, recipient, subject, body, booking_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		idgen.New(), domain.OutboxEmail, domain.OutboxRefundNotice, email, "We couldn't complete booking "+ref,
		fmt.Sprintf("We're sorry: your payment of %s reached us after your 10-minute hold ended, and the room had been booked by then. "+
			"We've started a full refund, and it should reach you in 5 to 7 working days. Our reservations team will also call you with other dates.",
			money.FormatINR(e.AmountPaise)), bookingID)
	return err
}

func (s *Service) confirmationMessages(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID) error {
	b, err := s.oneTx(ctx, tx, `b.id = $1`, bookingID)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Thank you, %s. Your booking %s is confirmed.\n\n%s, %s\n%s to %s (%d nights), %d room(s)\n"+
		"Paid: %s, including %s GST\n\nCheck-in from 2 pm, check-out by 11 am. Bring a photo ID; guests from outside India show their passport and visa at check-in.",
		firstName(b.Guest.Name), b.Ref, b.RoomTypeName, b.RatePlanName, b.CheckIn.Format("Mon 2 Jan"), b.CheckOut.Format("Mon 2 Jan 2006"),
		b.Nights(), b.Rooms, money.FormatINR(b.Quote.TotalPaise), money.FormatINR(b.Quote.TaxPaise))
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, recipient, subject, body, booking_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		idgen.New(), domain.OutboxEmail, domain.OutboxBookingConfirmation, b.Guest.Email, "Booking confirmed: "+b.Ref, body, bookingID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notifications (id, recipient_role, type, booking_id, body) VALUES ($1,$2,$3,$4,$5)`,
		idgen.New(), domain.RoleReservations, domain.NotifyBooking, bookingID,
		fmt.Sprintf("New direct booking %s: %s, %s", b.Ref, b.RoomTypeName, b.CheckIn.Format("2 Jan")))
	return err
}

// Sweep expires bookings whose hold ran out unpaid. Late payments within the grace period
// are still accepted (inventory.Confirm re-checks the room), so only older ones are closed.
func (s *Service) Sweep(ctx context.Context, grace time.Duration) (int64, error) {
	var n int64
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE bookings b SET status = $1, updated_at = now() FROM holds h
			WHERE h.id = b.hold_id AND b.status IN ($2, $3) AND h.expires_at < now() - make_interval(secs => $4)`,
			domain.BookingExpired, domain.BookingPendingPayment, domain.BookingPaymentFailed, grace.Seconds())
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		_, err = tx.Exec(ctx, `UPDATE holds SET status = $1 WHERE status = $2 AND expires_at < now() - make_interval(secs => $3)`,
			domain.HoldExpired, domain.HoldActive, grace.Seconds())
		return err
	})
	return n, err
}

// ---- validation

func validate(in *CreateInput) error {
	f := map[string]string{}
	in.Guest.Name = strings.Join(strings.Fields(in.Guest.Name), " ")
	if n := len([]rune(in.Guest.Name)); n < nameMin || n > nameMax {
		f["guest.name"] = fmt.Sprintf("must be %d to %d characters", nameMin, nameMax)
	}
	in.Guest.Email = strings.ToLower(strings.TrimSpace(in.Guest.Email))
	if a, err := mail.ParseAddress(in.Guest.Email); err != nil || a.Address != in.Guest.Email {
		f["guest.email"] = "must be a valid email address"
	}
	digits := 0
	for _, r := range in.Guest.Phone {
		switch {
		case unicode.IsDigit(r):
			digits++
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			digits = -100
		}
	}
	if digits < phoneMinDigits || digits > phoneMaxDigits {
		f["guest.phone"] = "must be a phone number with country code"
	}
	in.Guest.Country = strings.ToUpper(strings.TrimSpace(in.Guest.Country))
	if len(in.Guest.Country) != 2 || strings.IndexFunc(in.Guest.Country, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
		f["guest.country"] = "must be a two-letter country code"
	}
	if len([]rune(in.SpecialRequests)) > requestsMax {
		f["special_requests"] = fmt.Sprintf("must be at most %d characters", requestsMax)
	}
	if len([]rune(in.ArrivalWindow)) > arrivalWindowMax {
		f["arrival_window"] = "is too long"
	}
	if !in.Consent {
		f["consent"] = "is needed so we can use your details for this booking"
	}
	if in.DisplayCurrency != "" && !in.DisplayCurrency.Valid() {
		f["display_currency"] = "isn't a currency we show prices in"
	}
	if len(f) > 0 {
		return apperr.Validation(f)
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

func firstName(full string) string { return strings.SplitN(full, " ", 2)[0] }
