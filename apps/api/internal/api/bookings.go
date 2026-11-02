package api

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

const (
	headerContentDisposition = "Content-Disposition"
	contentTypeCSV           = "text/csv; charset=utf-8"
	maxWebhookBytes          = 16 << 10
)

// createBooking: POST /v1/bookings, with the hold's token in X-Hold-Token.
func (a *API) createBooking(w http.ResponseWriter, r *http.Request) error {
	var body CreateBookingRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	in := booking.CreateInput{
		HoldID: body.HoldID, HoldToken: r.Header.Get(headerHoldToken),
		Guest:         booking.Guest{Name: body.Guest.Name, Email: body.Guest.Email, Phone: body.Guest.Phone, Country: body.Guest.Country},
		ArrivalWindow: body.ArrivalWindow, SpecialRequests: body.SpecialRequests,
		Consent: body.Consent, MarketingOptIn: body.MarketingOptIn,
	}
	if p := principal(r.Context()); p.IsAuthenticated() && p.Role == domain.RoleGuest {
		in.GuestUserID = &p.UserID
	}
	b, err := a.bookings.Create(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, BookingResponse{Booking: bookingDTO(b)})
	return nil
}

// getBooking: GET /v1/bookings/{ref}. Visible to the browser that made it (hold token),
// the signed-in guest it belongs to, and staff. Anyone else gets "not found".
func (a *API) getBooking(w http.ResponseWriter, r *http.Request) error {
	b, err := a.bookings.ByRef(r.Context(), r.PathValue("ref"))
	if err != nil {
		return err
	}
	if !a.canSeeBooking(r, b) {
		return booking.ErrBookingNotFound
	}
	httpx.WriteJSON(w, http.StatusOK, BookingResponse{Booking: bookingDTO(b)})
	return nil
}

func (a *API) canSeeBooking(r *http.Request, b booking.Booking) bool {
	p := principal(r.Context())
	if p.IsAuthenticated() && auth.Can(p.Role, auth.PermBookingsRead) {
		return true
	}
	if p.IsAuthenticated() && p.Role == domain.RoleGuest {
		if u, err := a.auth.Me(r.Context(), p.UserID); err == nil && strings.EqualFold(u.Email, b.Guest.Email) {
			return true
		}
	}
	return a.bookings.HoldTokenMatches(r.Context(), b.ID, r.Header.Get(headerHoldToken))
}

// myBookings: GET /v1/me/bookings for a signed-in guest.
func (a *API) myBookings(w http.ResponseWriter, r *http.Request) error {
	p := principal(r.Context())
	u, err := a.auth.Me(r.Context(), p.UserID)
	if err != nil {
		return err
	}
	list, err := a.bookings.Mine(r.Context(), u.ID, u.Email)
	if err != nil {
		return err
	}
	writeBookings(w, list)
	return nil
}

// staffBookings: GET /v1/staff/bookings?status=&from=&to=&limit=
func (a *API) staffBookings(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	f := booking.ListFilter{Status: domain.BookingStatus(strings.ToUpper(p.String("status", ""))),
		Limit: p.Int("limit", booking.DefaultListLimit, 1, booking.MaxListLimit)}
	if r.URL.Query().Has("from") {
		f.From = p.Date("from")
	}
	if r.URL.Query().Has("to") {
		f.To = p.Date("to")
	}
	if err := p.Err(); err != nil {
		return err
	}
	if f.Status != "" && !f.Status.Valid() {
		return apperr.Validation(map[string]string{"status": "is not a booking status"})
	}
	list, err := a.bookings.List(r.Context(), f)
	if err != nil {
		return err
	}
	writeBookings(w, list)
	return nil
}

// exportBookings: GET /v1/staff/bookings/export?date=YYYY-MM-DD → CSV for the reservation system.
func (a *API) exportBookings(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	day := p.Date("date")
	if err := p.Err(); err != nil {
		return err
	}
	var buf bytes.Buffer // build fully first, so an error still gets a proper JSON response
	if err := a.bookings.ExportDay(r.Context(), &buf, day, istLocation()); err != nil {
		return err
	}
	w.Header().Set(httpx.HeaderContentType, contentTypeCSV)
	w.Header().Set(headerContentDisposition, `attachment; filename="kaapi-bookings-`+day.Format(time.DateOnly)+`.csv"`)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

// paymentPage: GET /v1/payments/{orderId}, what the mock gateway page shows.
func (a *API) paymentPage(w http.ResponseWriter, r *http.Request) error {
	b, err := a.bookings.ByOrder(r.Context(), r.PathValue("orderId"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, paymentPageDTO(b))
	return nil
}

// simulatePayment: POST /v1/payments/{orderId}/simulate. The mock provider builds and signs
// the webhook a real gateway would send, then delivers it through the same handler.
func (a *API) simulatePayment(w http.ResponseWriter, r *http.Request) error {
	var body SimulatePaymentRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	if body.Outcome != payment.OutcomeSuccess && body.Outcome != payment.OutcomeFailure {
		return apperr.Validation(map[string]string{"outcome": "must be SUCCESS or FAILURE"})
	}
	b, err := a.bookings.ByOrder(r.Context(), r.PathValue("orderId"))
	if err != nil {
		return err
	}
	raw, sig, err := payment.MockEvent(a.cfg.PaymentSecret, b.Payment.OrderID, b.Payment.AmountPaise, body.Outcome, time.Now())
	if err != nil {
		return err
	}
	if b, err = a.applyWebhook(r, raw, sig); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, paymentPageDTO(b))
	return nil
}

// paymentWebhook: POST /v1/payments/webhook, signed with HMAC in X-Mock-Signature.
func (a *API) paymentWebhook(w http.ResponseWriter, r *http.Request) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		return apperr.New(apperr.CodePayloadTooLarge, "Webhook body too large.")
	}
	if _, err := a.applyWebhook(r, raw, r.Header.Get(payment.HeaderSignature)); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"received": true})
	return nil
}

func (a *API) applyWebhook(r *http.Request, raw []byte, sig string) (booking.Booking, error) {
	e, err := payment.Verify(a.cfg.PaymentSecret, raw, sig)
	if err != nil {
		return booking.Booking{}, err
	}
	return a.bookings.HandlePaymentEvent(r.Context(), e)
}

func writeBookings(w http.ResponseWriter, list []booking.Booking) {
	out := BookingsResponse{Bookings: make([]BookingDTO, len(list))}
	for i, b := range list {
		out.Bookings[i] = bookingDTO(b)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// istLocation is the resort's time zone for "which day" questions.
func istLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Kolkata"); err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*3600+1800)
}
