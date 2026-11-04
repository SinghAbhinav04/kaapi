package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/platform/apperr"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

var guest = api.GuestDTO{Name: "Arjun Menon", Email: "arjun.menon@example.test", Phone: "+91 90000 10002", Country: "IN"}

// holdVilla holds the last Coffee Villa (breakfast) for 14–16 Nov, as on the design boards.
func holdVilla(t *testing.T, s *apitest.Server) api.HoldDTO {
	t.Helper()
	guestLogin(t, s, guest.Email) // only a guest with a verified email can book
	var room api.RoomResponse
	s.Do(http.MethodGet, "/v1/rooms/coffee-villa", nil, nil).JSON(t, &room)
	var planID = room.Room.Plans[1].ID // BB-FLEX
	var held api.HoldResponse
	res := s.Do(http.MethodPost, "/v1/holds", api.CreateHoldRequest{RoomTypeID: room.Room.ID, RatePlanID: planID,
		CheckIn: "2026-11-14", CheckOut: "2026-11-16", Adults: 2, Children: 1, PromoCode: "DIRECT10"}, nil)
	if res.Status != http.StatusCreated {
		t.Fatalf("hold: %d %s", res.Status, res.Body)
	}
	res.JSON(t, &held)
	return held.Hold
}

func book(t *testing.T, s *apitest.Server, h api.HoldDTO) api.BookingDTO {
	t.Helper()
	var out api.BookingResponse
	res := s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true,
		SpecialRequests: "Travelling with a small dog."}, map[string]string{"X-Hold-Token": h.Token})
	if res.Status != http.StatusCreated {
		t.Fatalf("book: %d %s", res.Status, res.Body)
	}
	res.JSON(t, &out)
	return out.Booking
}

func pay(t *testing.T, s *apitest.Server, orderID string, outcome payment.Outcome) api.PaymentPageDTO {
	t.Helper()
	var out api.PaymentPageDTO
	res := s.Do(http.MethodPost, "/v1/payments/"+orderID+"/simulate", api.SimulatePaymentRequest{Outcome: outcome}, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("pay: %d %s", res.Status, res.Body)
	}
	res.JSON(t, &out)
	return out
}

func villaLeft(t *testing.T, s *apitest.Server) int {
	t.Helper()
	var cal api.CalendarResponse
	s.Do(http.MethodGet, "/v1/rooms/coffee-villa/calendar?from=2026-11-14&to=2026-11-15", nil, nil).JSON(t, &cal)
	return cal.Days[0].Left
}

func TestBookAndPay(t *testing.T) {
	s := apitest.New(t)
	h := holdVilla(t, s)
	b := book(t, s, h)
	if b.Status != domain.BookingPendingPayment || b.Payment == nil || b.Quote.TotalPaise != 4_248_000 {
		t.Fatalf("%+v", b)
	}

	var page api.PaymentPageDTO
	s.Do(http.MethodGet, "/v1/payments/"+b.Payment.OrderID, nil, nil).JSON(t, &page)
	if page.AmountPaise != 4_248_000 || page.BookingRef != b.Ref {
		t.Fatalf("%+v", page)
	}

	paid := pay(t, s, b.Payment.OrderID, payment.OutcomeSuccess)
	if paid.BookingStatus != domain.BookingConfirmed || paid.Status != domain.PaymentCaptured {
		t.Fatalf("%+v", paid)
	}
	if left := villaLeft(t, s); left != 0 {
		t.Fatalf("the last villa should now be sold, %d left", left)
	}

	// Only the browser that booked (hold token), its guest or staff can see the booking.
	tok := map[string]string{"X-Hold-Token": h.Token}
	var got api.BookingResponse
	s.Do(http.MethodGet, "/v1/bookings/"+b.Ref, nil, tok).JSON(t, &got)
	if got.Booking.Status != domain.BookingConfirmed || got.Booking.ConfirmedAt == nil {
		t.Fatalf("%+v", got.Booking)
	}
	if e := s.NewSession().Do(http.MethodGet, "/v1/bookings/"+b.Ref, nil, nil).Error(t); e.Code != apperr.CodeNotFound {
		t.Fatalf("a stranger saw the booking: %s", e.Code)
	}
	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	if res := gm.Do(http.MethodGet, "/v1/bookings/"+b.Ref, nil, nil); res.Status != http.StatusOK {
		t.Fatalf("staff: %d", res.Status)
	}

	var out api.OutboxResponse
	gm.Do(http.MethodGet, "/v1/staff/outbox", nil, nil).JSON(t, &out)
	if m := out.Messages[0]; m.Kind != domain.OutboxBookingConfirmation || !strings.Contains(m.Body, b.Ref) || !strings.Contains(m.Body, "₹42,480") {
		t.Fatalf("confirmation email: %+v", m)
	}
}

// Without a verified email (signing in with a code) nobody can book: the confirmation would
// have nowhere real to go.
func TestBookingNeedsAVerifiedEmail(t *testing.T) {
	s := apitest.New(t)
	h := holdVilla(t, s)
	anon := s.NewSession()
	e := anon.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true},
		map[string]string{"X-Hold-Token": h.Token}).Error(t)
	if e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("anonymous booking: %s, want UNAUTHENTICATED", e.Code)
	}
	// A typed email that differs from the verified one is ignored.
	other := guest
	other.Email = "someone.else@example.test"
	var res api.BookingResponse
	s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: other, Consent: true},
		map[string]string{"X-Hold-Token": h.Token}).JSON(t, &res)
	if res.Booking.Guest.Email != guest.Email {
		t.Fatalf("booking email %q, want the verified %q", res.Booking.Guest.Email, guest.Email)
	}
}

func TestBookingIsIdempotentAndValidated(t *testing.T) {
	s := apitest.New(t)
	h := holdVilla(t, s)
	tok := map[string]string{"X-Hold-Token": h.Token}

	e := s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID,
		Guest: api.GuestDTO{Name: "A", Email: "nope", Phone: "123", Country: "India"}}, tok).Error(t)
	fields, _ := e.Details["fields"].(map[string]any)
	// No guest.email: the booking always uses the signed-in guest's verified address.
	for _, f := range []string{"guest.name", "guest.phone", "guest.country", "consent"} {
		if fields[f] == nil {
			t.Errorf("missing problem for %s: %+v", f, fields)
		}
	}

	if e := s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true},
		map[string]string{"X-Hold-Token": "stolen"}).Error(t); e.Code != apperr.CodeHoldNotFound {
		t.Fatalf("wrong token: %s", e.Code)
	}

	first, second := book(t, s, h), book(t, s, h)
	if first.Ref != second.Ref || first.Payment.OrderID != second.Payment.OrderID {
		t.Fatal("a double submit must return the same booking")
	}
}

func TestFailedPaymentCanBeRetried(t *testing.T) {
	s := apitest.New(t)
	b := book(t, s, holdVilla(t, s))
	failed := pay(t, s, b.Payment.OrderID, payment.OutcomeFailure)
	if failed.BookingStatus != domain.BookingPaymentFailed || failed.FailureReason == nil {
		t.Fatalf("%+v", failed)
	}
	if villaLeft(t, s) != 0 {
		t.Fatal("the hold should still keep the villa while the guest retries")
	}
	if ok := pay(t, s, b.Payment.OrderID, payment.OutcomeSuccess); ok.BookingStatus != domain.BookingConfirmed {
		t.Fatalf("retry: %+v", ok)
	}
}

func TestWebhookSignatureAndReplay(t *testing.T) {
	s := apitest.New(t)
	b := book(t, s, holdVilla(t, s))
	secret := apitest.TestConfig().PaymentSecret
	raw, sig, err := payment.MockEvent(secret, b.Payment.OrderID, b.Payment.AmountPaise, payment.OutcomeSuccess, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// A correctly signed event for the wrong amount is refused while the payment is pending.
	cheaper, csig, err := payment.MockEvent(secret, b.Payment.OrderID, 100, payment.OutcomeSuccess, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if e := s.DoRaw(http.MethodPost, "/v1/payments/webhook", cheaper, map[string]string{payment.HeaderSignature: csig}).Error(t); e.Code != apperr.CodeConflict {
		t.Fatalf("wrong amount: %s", e.Code)
	}

	forged := map[string]string{payment.HeaderSignature: strings.Repeat("ab", 32)}
	if e := s.DoRaw(http.MethodPost, "/v1/payments/webhook", raw, forged).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("forged signature: %s", e.Code)
	}
	signed := map[string]string{payment.HeaderSignature: sig}
	for i := 0; i < 3; i++ { // providers retry; the same event must only ever sell once
		if res := s.DoRaw(http.MethodPost, "/v1/payments/webhook", raw, signed); res.Status != http.StatusOK {
			t.Fatalf("delivery %d: %d %s", i+1, res.Status, res.Body)
		}
	}
	var n int
	if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) FROM booking_nights bn JOIN bookings b ON b.id = bn.booking_id WHERE b.ref = $1`, b.Ref).Scan(&n); err != nil || n != 2 {
		t.Fatalf("booked nights %d (%v), want 2", n, err)
	}

}

func TestLatePaymentAfterRoomWasTakenIsRefunded(t *testing.T) {
	s := apitest.New(t)
	h := holdVilla(t, s)
	b := book(t, s, h)

	// The guest's 10 minutes run out, and someone else takes the last villa.
	if _, err := s.Pool.Exec(s.Ctx, `UPDATE holds SET expires_at = now() - interval '1 minute' WHERE id = $1`, h.ID); err != nil {
		t.Fatal(err)
	}
	other := s.NewSession()
	holdVilla(t, other)

	late := pay(t, s, b.Payment.OrderID, payment.OutcomeSuccess)
	if late.BookingStatus != domain.BookingRefundPending || late.Status != domain.PaymentRefundPending {
		t.Fatalf("%+v", late)
	}
	var alerts int
	if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) FROM notifications WHERE type = $1 AND recipient_role = $2`,
		domain.NotifyRefundNeeded, domain.RoleGM).Scan(&alerts); err != nil || alerts != 1 {
		t.Fatalf("GM refund alerts: %d %v", alerts, err)
	}
}

func TestGuestSeesTheirBookings(t *testing.T) {
	s := apitest.New(t)
	guestLogin(t, s, "guest@example.test") // Meera Iyer has two seeded bookings
	var mine api.BookingsResponse
	s.Do(http.MethodGet, "/v1/me/bookings", nil, nil).JSON(t, &mine)
	if len(mine.Bookings) != 2 {
		t.Fatalf("%d bookings", len(mine.Bookings))
	}
	if res := s.Do(http.MethodGet, "/v1/bookings/"+mine.Bookings[0].Ref, nil, nil); res.Status != http.StatusOK {
		t.Fatalf("own booking by ref: %d", res.Status)
	}
	if e := s.Do(http.MethodGet, "/v1/staff/bookings", nil, nil).Error(t); e.Code != apperr.CodeForbidden {
		t.Fatalf("guest listing all bookings: %s", e.Code)
	}
}

func TestStaffListAndExport(t *testing.T) {
	s := apitest.New(t)
	b := book(t, s, holdVilla(t, s))
	pay(t, s, b.Payment.OrderID, payment.OutcomeSuccess)

	res := s.NewSession()
	staffLogin(t, res, "reservations@kaapiestate.test", nil)
	var list api.BookingsResponse
	res.Do(http.MethodGet, "/v1/staff/bookings?status=confirmed&from=2026-11-01&to=2026-12-01", nil, nil).JSON(t, &list)
	found := false
	for _, x := range list.Bookings {
		found = found || x.Ref == b.Ref
		if x.Status != domain.BookingConfirmed {
			t.Fatalf("filter leaked %s", x.Status)
		}
	}
	if !found {
		t.Fatal("new booking missing from the list")
	}

	today := time.Now().In(time.FixedZone("IST", 5*3600+1800)).Format(time.DateOnly)
	csv := res.Do(http.MethodGet, "/v1/staff/bookings/export?date="+today, nil, nil)
	if csv.Status != http.StatusOK || !strings.HasPrefix(csv.Header.Get("Content-Type"), "text/csv") ||
		!strings.Contains(string(csv.Body), b.Ref) || !strings.HasPrefix(string(csv.Body), "booking_ref,status,") {
		t.Fatalf("%d %s", csv.Status, csv.Body)
	}

	sales := s.NewSession()
	staffLogin(t, sales, "sales@kaapiestate.test", nil)
	if e := sales.Do(http.MethodGet, "/v1/staff/bookings/export?date="+today, nil, nil).Error(t); e.Code != apperr.CodeForbidden {
		t.Fatalf("sales exporting: %s", e.Code)
	}
}
