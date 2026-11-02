// Package payment is the payment-provider boundary: webhook event shapes and their HMAC
// signatures. The mock provider builds events exactly as a real one (Razorpay, Stripe)
// would, so swapping it changes this package only, not the booking flow.
package payment

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
)

// EventType is what happened to a payment.
type EventType string

const (
	EventCaptured EventType = "payment.captured"
	EventFailed   EventType = "payment.failed"
)

// Outcome is what a tester chooses on the mock payment page.
type Outcome string

const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeFailure Outcome = "FAILURE"
)

const (
	// HeaderSignature carries hex(HMAC-SHA256(secret, body)).
	HeaderSignature = "X-Mock-Signature"
	orderIDBytes    = 12
	orderIDPrefix   = "order_"
	paymentIDPrefix = "pay_mock_"
	eventIDPrefix   = "evt_"
	mockDeclineText = "The bank declined this test payment."
)

// Event is a webhook delivery.
type Event struct {
	ID            string                 `json:"id"`
	Type          EventType              `json:"type"`
	Provider      domain.PaymentProvider `json:"provider"`
	OrderID       string                 `json:"order_id"`
	PaymentID     string                 `json:"payment_id"`
	AmountPaise   int64                  `json:"amount_paise"`
	FailureReason string                 `json:"failure_reason,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

var ErrBadSignature = apperr.New(apperr.CodeUnauthenticated, "The webhook signature is not valid.")

// Sign returns the signature for a raw body.
func Sign(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Verify checks a signature in constant time and decodes the event.
func Verify(secret, body []byte, signature string) (Event, error) {
	want, err := hex.DecodeString(signature)
	if err != nil {
		return Event{}, ErrBadSignature
	}
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	if !hmac.Equal(m.Sum(nil), want) {
		return Event{}, ErrBadSignature
	}
	var e Event
	if err := json.Unmarshal(body, &e); err != nil || e.ID == "" || e.OrderID == "" {
		return Event{}, apperr.New(apperr.CodeBadRequest, "The webhook body is not a payment event.")
	}
	return e, nil
}

// NewOrderID returns an unguessable order id.
func NewOrderID() string { return orderIDPrefix + randomHex(orderIDBytes) }

// MockEvent builds the signed delivery the mock provider would send for an outcome.
func MockEvent(secret []byte, orderID string, amount int64, outcome Outcome, now time.Time) ([]byte, string, error) {
	e := Event{ID: eventIDPrefix + randomHex(orderIDBytes), Provider: domain.PaymentProviderMock, OrderID: orderID,
		PaymentID: paymentIDPrefix + randomHex(orderIDBytes/2), AmountPaise: amount, CreatedAt: now, Type: EventCaptured}
	if outcome == OutcomeFailure {
		e.Type, e.FailureReason = EventFailed, mockDeclineText
	}
	body, err := json.Marshal(e)
	if err != nil {
		return nil, "", err
	}
	return body, Sign(secret, body), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
