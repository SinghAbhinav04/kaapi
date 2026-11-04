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
	pool     *pgxpool.Pool
	store    store
	now      func() time.Time
	consents Consents // nil: the offers opt-in is only kept on the booking
}

// Consents records a guest's opt-in to offer emails (package campaign).
type Consents interface {
	OptIn(ctx context.Context, q db.Querier, userID uuid.UUID) error
}

// SetConsents wires the offers opt-in; the app does it once.
func (s *Service) SetConsents(c Consents) { s.consents = c }

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
		h, err := s.store.lockHold(ctx, tx, in.HoldID)
		if errors.Is(err, pgx.ErrNoRows) {
			return inventory.ErrHoldNotFound
		}
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(in.HoldToken))
		if subtle.ConstantTimeCompare(sum[:], h.tokenHash) != 1 {
			return inventory.ErrHoldNotFound
		}
		// Idempotent: a double-click or a retried request gets the booking already made.
		bookingID, err = s.store.bookingForHold(ctx, tx, in.HoldID)
		if err == nil || !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if h.status != domain.HoldActive || !h.live {
			return inventory.ErrHoldNotActive
		}
		var q pricing.Quote
		if err := json.Unmarshal(h.quoteJSON, &q); err != nil {
			return fmt.Errorf("hold quote: %w", err)
		}

		// What the guest was reading prices in, with that moment's rate. Never part of the charge.
		nb := newBooking{holdID: in.HoldID, in: in, hold: h, quote: q, at: s.now()}
		if in.DisplayCurrency != "" {
			nb.shown = &in.DisplayCurrency
			if nb.rate, err = fx.Snapshot(ctx, tx, in.DisplayCurrency); err != nil {
				return err
			}
		}
		for attempt := 0; attempt < refAttempts; attempt++ {
			nb.id, nb.ref = idgen.New(), idgen.BookingRef()
			ok, err := s.store.insertBooking(ctx, tx, nb)
			if err != nil {
				return err
			}
			if ok {
				bookingID = nb.id
				break
			}
		}
		if bookingID == uuid.Nil {
			return errors.New("booking: could not allocate a unique reference")
		}
		if in.MarketingOptIn && in.GuestUserID != nil && s.consents != nil {
			if err := s.consents.OptIn(ctx, tx, *in.GuestUserID); err != nil {
				return err
			}
		}
		return s.store.insertPaymentOrder(ctx, tx, bookingID, q.TotalPaise, in.HoldID.String())
	})
	if err != nil {
		return Booking{}, err
	}
	return s.store.one(ctx, s.pool, `b.id = $1`, bookingID)
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
		o, err := s.store.lockOrder(ctx, tx, e.OrderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		bookingID = o.bookingID
		fresh, err := s.store.recordEvent(ctx, tx, e, payload)
		if err != nil {
			return err
		}
		if !fresh || o.status == domain.PaymentCaptured {
			return nil // already processed, or already paid by an earlier event
		}
		if e.AmountPaise != o.amount {
			return ErrAmountMismatch
		}
		var holdID uuid.UUID
		if o.holdID != nil {
			holdID = *o.holdID
		}

		switch e.Type {
		case payment.EventFailed:
			if err := s.store.setPayment(ctx, tx, o.paymentID, domain.PaymentFailed, e.PaymentID, &e.FailureReason); err != nil {
				return err
			}
			return s.store.setBookingStatus(ctx, tx, bookingID, domain.BookingPaymentFailed)

		case payment.EventCaptured:
			err := inventory.Confirm(ctx, tx, holdID, bookingID)
			if errors.Is(err, inventory.ErrSoldOut) || errors.Is(err, inventory.ErrHoldNotActive) {
				return s.refundPath(ctx, tx, o, e)
			}
			if err != nil {
				return err // rolled back; the provider retries the delivery
			}
			if err := s.store.setPayment(ctx, tx, o.paymentID, domain.PaymentCaptured, e.PaymentID, nil); err != nil {
				return err
			}
			if err := s.store.setBookingStatus(ctx, tx, bookingID, domain.BookingConfirmed); err != nil {
				return err
			}
			if o.promoID != nil {
				if err := s.store.usePromo(ctx, tx, *o.promoID); err != nil {
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
	return s.store.one(ctx, s.pool, `b.id = $1`, bookingID)
}

// refundPath: the payment arrived after the room was taken. Mark it for refund, alert the
// GM and tell the guest.
func (s *Service) refundPath(ctx context.Context, tx pgx.Tx, o orderRow, e payment.Event) error {
	if err := s.store.setPayment(ctx, tx, o.paymentID, domain.PaymentRefundPending, e.PaymentID, nil); err != nil {
		return err
	}
	if err := s.store.setBookingStatus(ctx, tx, o.bookingID, domain.BookingRefundPending); err != nil {
		return err
	}
	if err := s.store.notify(ctx, tx, domain.RoleGM, domain.NotifyRefundNeeded, o.bookingID,
		fmt.Sprintf("Refund %s for %s: the payment arrived after the room was taken.", money.FormatINR(e.AmountPaise), o.ref)); err != nil {
		return err
	}
	return s.store.queueEmail(ctx, tx, domain.OutboxRefundNotice, string(o.guestEmail), "We couldn't complete booking "+o.ref,
		fmt.Sprintf("We're sorry: your payment of %s reached us after your 10-minute hold ended, and the room had been booked by then. "+
			"We've started a full refund, and it should reach you in 5 to 7 working days. Our reservations team will also call you with other dates.",
			money.FormatINR(e.AmountPaise)), o.bookingID)
}

func (s *Service) confirmationMessages(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID) error {
	b, err := s.store.one(ctx, tx, `b.id = $1`, bookingID)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Thank you, %s. Your booking %s is confirmed.\n\n%s, %s\n%s to %s (%d nights), %d room(s)\n"+
		"Paid: %s, including %s GST\n\nCheck-in from 2 pm, check-out by 11 am. Bring a photo ID; guests from outside India show their passport and visa at check-in.",
		firstName(b.Guest.Name), b.Ref, b.RoomTypeName, b.RatePlanName, b.CheckIn.Format("Mon 2 Jan"), b.CheckOut.Format("Mon 2 Jan 2006"),
		b.Nights(), b.Rooms, money.FormatINR(b.Quote.TotalPaise), money.FormatINR(b.Quote.TaxPaise))
	// Queued in the payment's transaction, so a confirmed booking always gets its email.
	if err := s.store.queueEmail(ctx, tx, domain.OutboxBookingConfirmation, b.Guest.Email, "Booking confirmed: "+b.Ref, body, bookingID); err != nil {
		return err
	}
	return s.store.notify(ctx, tx, domain.RoleReservations, domain.NotifyBooking, bookingID,
		fmt.Sprintf("New direct booking %s: %s, %s", b.Ref, b.RoomTypeName, b.CheckIn.Format("2 Jan")))
}

// Sweep expires bookings whose hold ran out unpaid. Late payments within the grace period
// are still accepted (inventory.Confirm re-checks the room), so only older ones are closed.
func (s *Service) Sweep(ctx context.Context, grace time.Duration) (int64, error) {
	var n int64
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if n, err = s.store.expireUnpaid(ctx, tx, grace); err != nil {
			return err
		}
		return s.store.expireHolds(ctx, tx, grace)
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
