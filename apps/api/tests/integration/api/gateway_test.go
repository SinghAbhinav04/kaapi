package api_test

import (
	"net/http"
	"strconv"
	"testing"

	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/config"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

const nativeOrigin = "capacitor://localhost"

// withGateway turns on the limits and puts the API behind one proxy, as the web app runs it,
// so each test can speak as a different client address through X-Forwarded-For.
func withGateway(t *testing.T) *apitest.Server {
	t.Helper()
	cfg := apitest.TestConfig()
	cfg.RateLimits = true
	cfg.TrustedProxyHops = 1
	cfg.NativeAppOrigins = []string{nativeOrigin}
	return apitest.NewWith(t, cfg)
}

func from(ip string) map[string]string { return map[string]string{"X-Forwarded-For": ip} }

func TestHoldsAreRateLimitedPerClientIP(t *testing.T) {
	s := withGateway(t)
	// A hold locks a room for ten minutes, so 20 attempts per address per 10 minutes, valid or not.
	for i := 0; i < 20; i++ {
		if res := s.Do(http.MethodPost, "/v1/holds", map[string]any{}, from("203.0.113.7")); res.Status == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was limited too early", i+1)
		}
	}
	res := s.Do(http.MethodPost, "/v1/holds", map[string]any{}, from("203.0.113.7"))
	e := res.Error(t)
	if res.Status != http.StatusTooManyRequests || e.Code != apperr.CodeRateLimited {
		t.Fatalf("21st attempt: %d %s", res.Status, e.Code)
	}
	if secs, err := strconv.Atoi(res.Header.Get("Retry-After")); err != nil || secs < 1 || secs > 600 {
		t.Fatalf("Retry-After: %q", res.Header.Get("Retry-After"))
	}
	if res := s.Do(http.MethodPost, "/v1/holds", map[string]any{}, from("198.51.100.20")); res.Status == http.StatusTooManyRequests {
		t.Fatal("another address must have its own allowance")
	}
	// A forged leftmost entry doesn't buy a fresh allowance: the proxy's entry is the one used.
	if res := s.Do(http.MethodPost, "/v1/holds", map[string]any{}, from("1.2.3.4, 203.0.113.7")); res.Status != http.StatusTooManyRequests {
		t.Fatalf("forged X-Forwarded-For escaped the limit: %d", res.Status)
	}
}

func TestStaffLoginIsLimitedEvenWithTheRightPassword(t *testing.T) {
	s := withGateway(t)
	for i := 0; i < 10; i++ {
		s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "gm@kaapiestate.test", Password: "guess"}, from("203.0.113.9"))
	}
	res := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "gm@kaapiestate.test", Password: staffPassword}, from("203.0.113.9"))
	if res.Status != http.StatusTooManyRequests {
		t.Fatalf("11th login attempt from one address: %d", res.Status)
	}
}

func TestLogoutEndsTheAccessTokenAtOnce(t *testing.T) {
	s := apitest.New(t)
	login := staffLogin(t, s.NewSession(), "reservations@kaapiestate.test", native)
	app := s.NewSession()
	bearer := map[string]string{"Authorization": "Bearer " + *login.AccessToken}
	if res := app.Do(http.MethodGet, "/v1/me", nil, bearer); res.Status != http.StatusOK {
		t.Fatalf("before logout: %d", res.Status)
	}
	if res := app.Do(http.MethodPost, "/v1/auth/logout", nil, bearer); res.Status != http.StatusNoContent {
		t.Fatalf("logout: %d", res.Status)
	}
	e := app.Do(http.MethodGet, "/v1/me", nil, bearer).Error(t)
	if e.Code != apperr.CodeUnauthenticated || e.Message == "Please sign in." {
		t.Fatalf("the token must die with its session, not live out its 15 minutes: %+v", e)
	}
}

func TestRefreshRetiresTheOldAccessToken(t *testing.T) {
	s := apitest.New(t)
	login := staffLogin(t, s.NewSession(), "sales@kaapiestate.test", native)
	app := s.NewSession()
	var rotated api.AuthResponse
	app.Do(http.MethodPost, "/v1/auth/refresh", api.RefreshRequest{RefreshToken: *login.RefreshToken}, native).JSON(t, &rotated)

	old := map[string]string{"Authorization": "Bearer " + *login.AccessToken}
	if res := app.Do(http.MethodGet, "/v1/me", nil, old); res.Status != http.StatusUnauthorized {
		t.Fatalf("the replaced token still works: %d", res.Status)
	}
	if res := app.Do(http.MethodGet, "/v1/me", nil, map[string]string{"Authorization": "Bearer " + *rotated.AccessToken}); res.Status != http.StatusOK {
		t.Fatalf("the new token: %d", res.Status)
	}
}

// preflight sends a browser's CORS preflight straight to the server (preflights aren't
// operations in the contract, so they skip the contract check).
func preflight(t *testing.T, s *apitest.Server, method, path, origin string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(s.Ctx, http.MethodOptions, s.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.Header.Get("Access-Control-Allow-Origin")
}

func TestCORSIsDecidedPerGroup(t *testing.T) {
	s := withGateway(t)
	app := apitest.TestConfig().AppOrigin
	cases := []struct {
		method, path, origin, want string
	}{
		{http.MethodGet, "/v1/rooms", nativeOrigin, nativeOrigin},             // public: web app and native shells
		{http.MethodPost, "/v1/auth/otp/request", nativeOrigin, nativeOrigin}, // guests sign in from the app
		{http.MethodGet, "/v1/staff/bookings", app, app},                      // staff console: the web app only
		{http.MethodGet, "/v1/staff/bookings", nativeOrigin, ""},
		{http.MethodPost, "/v1/payments/webhook", app, ""}, // server to server: no CORS for anyone
		{http.MethodGet, "/v1/rooms", "https://evil.example", ""},
		{http.MethodGet, "/healthz", app, ""}, // outside the gateway
	}
	for _, c := range cases {
		if got := preflight(t, s, c.method, c.path, c.origin); got != c.want {
			t.Errorf("%s %s from %s: allowed %q, want %q", c.method, c.path, c.origin, got, c.want)
		}
	}
}

// Rate limits are on by default and the proxy count defaults to the web app's one hop.
func TestGatewayDefaults(t *testing.T) {
	t.Setenv(config.EnvDatabaseURL, "postgres://unused")
	t.Setenv(config.EnvJWESecret, "0123456789abcdef0123456789abcdef")
	t.Setenv(config.EnvPaymentSecret, "0123456789abcdef0123456789abcdef")
	c, err := config.Load()
	if err != nil || !c.RateLimits || c.TrustedProxyHops != 1 {
		t.Fatalf("limits %v, hops %d, err %v", c.RateLimits, c.TrustedProxyHops, err)
	}
}
