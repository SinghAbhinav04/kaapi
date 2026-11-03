package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/platform/apperr"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// fakeFrankfurter answers /latest like the ECB feed: units per EUR, with AUD missing that day.
func fakeFrankfurter(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/latest" || r.URL.Query().Get("symbols") == "" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"amount":1.0,"base":"EUR","date":"2026-11-02",
			"rates":{"INR":100,"USD":1.25,"GBP":0.8,"JPY":160,"CAD":3,"SGD":1.5,"CHF":0.95}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func withFX(t *testing.T) *apitest.Server {
	t.Helper()
	cfg := apitest.TestConfig()
	cfg.FXBaseURL = fakeFrankfurter(t).URL
	s := apitest.NewWith(t, cfg)
	// A reseed keeps fetched rates (they aren't demo data, like the Gmail connection), so
	// start each test from none.
	if _, err := s.Pool.Exec(s.Ctx, `DELETE FROM fx_rates`); err != nil {
		t.Fatal(err)
	}
	return s
}

func fxRates(t *testing.T, s *apitest.Server) map[domain.Currency]int64 {
	t.Helper()
	var out api.FxRatesResponse
	s.Do(http.MethodGet, "/v1/fx-rates", nil, nil).JSON(t, &out)
	if out.ChargedIn != domain.CurrencyINR {
		t.Fatalf("charged in %s", out.ChargedIn)
	}
	m := map[domain.Currency]int64{}
	for _, r := range out.Rates {
		m[r.Currency] = r.INRPerUnitMicros
	}
	return m
}

func TestFXRatesAreCrossedThroughEURExactly(t *testing.T) {
	s := withFX(t)
	if got := fxRates(t, s); len(got) != 0 {
		t.Fatalf("no rates fetched yet, so none shown: %v", got)
	}
	if n, err := s.App.FX.Refresh(s.Ctx); err != nil || n != 7 {
		t.Fatalf("refresh: %d %v", n, err)
	}
	want := map[domain.Currency]int64{
		domain.CurrencyEUR: 100_000_000, // 100 INR per EUR
		domain.CurrencyUSD: 80_000_000,  // 100 / 1.25
		domain.CurrencyGBP: 125_000_000,
		domain.CurrencyJPY: 625_000,    // 0.625 INR per yen
		domain.CurrencyCAD: 33_333_333, // 33.3333333…, rounded once
		domain.CurrencySGD: 66_666_667, // 66.6666666…, rounded half up
		domain.CurrencyCHF: 105_263_158,
	}
	got := fxRates(t, s)
	for c, micros := range want {
		if got[c] != micros {
			t.Errorf("%s: got %d, want %d", c, got[c], micros)
		}
	}
	if _, ok := got[domain.CurrencyAUD]; ok {
		t.Error("AUD wasn't published, so it must not appear")
	}

	// Fetching has been failing for two days: show INR only rather than a drifted estimate.
	if _, err := s.Pool.Exec(s.Ctx, `UPDATE fx_rates SET fetched_at = now() - interval '49 hours'`); err != nil {
		t.Fatal(err)
	}
	if got := fxRates(t, s); len(got) != 0 {
		t.Fatalf("stale rates must be hidden: %v", got)
	}
}

func TestBookingKeepsTheCurrencyTheGuestSaw(t *testing.T) {
	s := withFX(t)
	if _, err := s.App.FX.Refresh(s.Ctx); err != nil {
		t.Fatal(err)
	}
	h := holdVilla(t, s)
	token := map[string]string{"X-Hold-Token": h.Token}

	e := s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true,
		DisplayCurrency: "XYZ"}, token).Error(t)
	if e.Code != apperr.CodeValidation {
		t.Fatalf("unknown currency: %s", e.Code)
	}

	var res api.BookingResponse
	s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true,
		DisplayCurrency: domain.CurrencyUSD}, token).JSON(t, &res)
	b := res.Booking
	if b.Quote.TotalPaise != 4_248_000 || b.Payment.AmountPaise != 4_248_000 {
		t.Fatalf("the charge must stay the INR quote: %+v", b.Quote)
	}
	var shown string
	var micros int64
	var rateDate string
	if err := s.Pool.QueryRow(s.Ctx, `SELECT display_currency, fx_inr_per_unit_micros, fx_rate_date::text FROM bookings WHERE ref = $1`,
		b.Ref).Scan(&shown, &micros, &rateDate); err != nil {
		t.Fatal(err)
	}
	if shown != "USD" || micros != 80_000_000 || rateDate != "2026-11-02" {
		t.Fatalf("snapshot: %s %d %s", shown, micros, rateDate)
	}

	// Analytics counts it under the currency shown; seeded bookings, with none, count as INR.
	pay(t, s, b.Payment.OrderID, payment.OutcomeSuccess)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	var rep api.AnalyticsResponse
	s.Do(http.MethodGet, "/v1/staff/analytics?from=2026-01-01&to=2026-12-31", nil, nil).JSON(t, &rep)
	byCurrency := map[string]api.AnalyticsBreakdownDTO{}
	for _, row := range rep.ByCurrency {
		byCurrency[row.Name] = row
	}
	if usd := byCurrency["USD"]; usd.Bookings != 1 || usd.RevenuePaise != 4_248_000 {
		t.Fatalf("USD row: %+v", usd)
	}
	if byCurrency["INR"].Bookings == 0 {
		t.Fatalf("seeded bookings should count as INR: %+v", rep.ByCurrency)
	}
}

func TestStaleRatesStillRecordTheCurrencyButNoRate(t *testing.T) {
	s := withFX(t) // never refreshed: no rates at all
	h := holdVilla(t, s)
	var res api.BookingResponse
	s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true,
		DisplayCurrency: domain.CurrencyGBP}, map[string]string{"X-Hold-Token": h.Token}).JSON(t, &res)
	var shown string
	var micros *int64
	if err := s.Pool.QueryRow(s.Ctx, `SELECT display_currency, fx_inr_per_unit_micros FROM bookings WHERE ref = $1`,
		res.Booking.Ref).Scan(&shown, &micros); err != nil {
		t.Fatal(err)
	}
	if shown != "GBP" || micros != nil {
		t.Fatalf("got %s %v", shown, micros)
	}
}
