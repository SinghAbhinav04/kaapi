package api_test

import (
	"net/http"
	"testing"

	"kaapi/api/internal/api"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/tests/apitest"
)

// testdb seeds every staff account with this password.
const staffPassword = "test-only-password"

var native = map[string]string{"X-Client": "native"}

func staffLogin(t *testing.T, s *apitest.Server, email string, headers map[string]string) api.AuthResponse {
	t.Helper()
	var out api.AuthResponse
	res := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: email, Password: staffPassword}, headers)
	if res.Status != http.StatusOK {
		t.Fatalf("login %s: %d %s", email, res.Status, res.Body)
	}
	res.JSON(t, &out)
	return out
}

func guestLogin(t *testing.T, s *apitest.Server, email string) api.AuthResponse {
	t.Helper()
	var sent api.OTPRequestResponse
	s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: email}, nil).JSON(t, &sent)
	if sent.DemoCode == nil {
		t.Fatal("demo mode should return the code")
	}
	var out api.AuthResponse
	res := s.Do(http.MethodPost, "/v1/auth/otp/verify", api.OTPVerifyRequest{Email: email, Code: *sent.DemoCode}, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("verify: %d %s", res.Status, res.Body)
	}
	res.JSON(t, &out)
	return out
}

func TestStaffSignInAndPermissions(t *testing.T) {
	s := apitest.New(t)

	if e := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "gm@kaapiestate.test", Password: "wrong"}, nil).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("wrong password: %s", e.Code)
	}
	if e := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "nobody@kaapiestate.test", Password: staffPassword}, nil).Error(t); e.Message != "That email and password don't match." {
		t.Fatalf("unknown email must look the same as a wrong password: %q", e.Message)
	}

	gm := staffLogin(t, s, "GM@KaapiEstate.test", nil) // emails are case-insensitive
	if gm.User.Role != domain.RoleGM || gm.AccessToken != nil {
		t.Fatalf("web login must not put tokens in the body: %+v", gm)
	}
	if res := s.Do(http.MethodGet, "/v1/me", nil, nil); res.Status != http.StatusOK {
		t.Fatalf("cookie session: %d", res.Status)
	}
	if res := s.Do(http.MethodGet, "/v1/staff/outbox", nil, nil); res.Status != http.StatusOK {
		t.Fatalf("GM reads the outbox: %d", res.Status)
	}
}

func TestGuestOTPSignIn(t *testing.T) {
	s := apitest.New(t)
	email := "Arjun.Menon@Example.test"

	if e := s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: "not-an-email"}, nil).Error(t); e.Code != apperr.CodeValidation {
		t.Fatalf("bad email: %s", e.Code)
	}

	var sent api.OTPRequestResponse
	s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: email}, nil).JSON(t, &sent)
	wrong := "000000"
	if *sent.DemoCode == wrong {
		wrong = "111111"
	}
	if e := s.Do(http.MethodPost, "/v1/auth/otp/verify", api.OTPVerifyRequest{Email: email, Code: wrong}, nil).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("wrong code: %s", e.Code)
	}
	var out api.AuthResponse
	s.Do(http.MethodPost, "/v1/auth/otp/verify", api.OTPVerifyRequest{Email: email, Code: *sent.DemoCode}, nil).JSON(t, &out)
	if out.User.Role != domain.RoleGuest || out.User.Email != "arjun.menon@example.test" {
		t.Fatalf("%+v", out.User)
	}
	if e := s.Do(http.MethodGet, "/v1/staff/outbox", nil, nil).Error(t); e.Code != apperr.CodeForbidden {
		t.Fatalf("guest reading staff outbox: %s", e.Code)
	}
	if e := s.Do(http.MethodPost, "/v1/auth/otp/verify", api.OTPVerifyRequest{Email: email, Code: *sent.DemoCode}, nil).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatal("a code must only work once")
	}
}

func TestOTPCannotGrantStaffAccess(t *testing.T) {
	s := apitest.New(t)
	var sent api.OTPRequestResponse
	s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: "gm@kaapiestate.test"}, nil).JSON(t, &sent)
	if e := s.Do(http.MethodPost, "/v1/auth/otp/verify", api.OTPVerifyRequest{Email: "gm@kaapiestate.test", Code: *sent.DemoCode}, nil).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("a staff email signed in with a code: %s", e.Code)
	}
}

func TestOTPRateLimit(t *testing.T) {
	s := apitest.New(t)
	for i := 0; i < 3; i++ {
		if res := s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: "busy@example.test"}, nil); res.Status != http.StatusAccepted {
			t.Fatalf("request %d: %d", i+1, res.Status)
		}
	}
	if e := s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: "busy@example.test"}, nil).Error(t); e.Code != apperr.CodeRateLimited {
		t.Fatalf("4th request: %s", e.Code)
	}
}

func TestNativeTokensAndRefreshRotation(t *testing.T) {
	s := apitest.New(t)
	login := staffLogin(t, s.NewSession(), "reservations@kaapiestate.test", native)
	if login.AccessToken == nil || login.RefreshToken == nil {
		t.Fatal("native login must return tokens")
	}

	app := s.NewSession() // no cookies: Bearer only
	bearer := map[string]string{"Authorization": "Bearer " + *login.AccessToken}
	if res := app.Do(http.MethodGet, "/v1/me", nil, bearer); res.Status != http.StatusOK {
		t.Fatalf("bearer: %d", res.Status)
	}

	var rotated api.AuthResponse
	res := app.Do(http.MethodPost, "/v1/auth/refresh", api.RefreshRequest{RefreshToken: *login.RefreshToken}, native)
	res.JSON(t, &rotated)
	if res.Status != http.StatusOK || *rotated.RefreshToken == *login.RefreshToken {
		t.Fatalf("rotation: %d", res.Status)
	}

	// Someone replays the old refresh token: every session of that user is revoked.
	if e := app.Do(http.MethodPost, "/v1/auth/refresh", api.RefreshRequest{RefreshToken: *login.RefreshToken}, native).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("reuse: %s", e.Code)
	}
	if e := app.Do(http.MethodPost, "/v1/auth/refresh", api.RefreshRequest{RefreshToken: *rotated.RefreshToken}, native).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("after reuse the newest token must be dead too: %s", e.Code)
	}
}

// The web keeps tokens in cookies and renews through the same endpoint, with no body.
func TestWebRefreshRotatesCookies(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "admin@kaapiestate.test", nil)
	origin := map[string]string{"Origin": apitest.TestConfig().AppOrigin}
	res := s.Do(http.MethodPost, "/v1/auth/refresh", nil, origin)
	if res.Status != http.StatusOK {
		t.Fatalf("cookie refresh: %d %s", res.Status, res.Body)
	}
	if got := len(res.Header.Values("Set-Cookie")); got != 2 {
		t.Fatalf("refresh must set both cookies again, set %d", got)
	}
	if res := s.Do(http.MethodGet, "/v1/me", nil, nil); res.Status != http.StatusOK {
		t.Fatalf("after refresh: %d", res.Status)
	}
}

// Browsers reach the API through the web app's /api proxy, so the refresh cookie's path must
// be the one the browser sees; with the bare /v1/auth it was never sent and every staff
// session ended after 15 minutes.
func TestRefreshCookiePathMatchesTheBrowserProxy(t *testing.T) {
	cfg := apitest.TestConfig()
	cfg.BrowserAPIPrefix = "/api"
	s := apitest.NewWith(t, cfg)
	res := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "gm@kaapiestate.test", Password: staffPassword}, nil)
	paths := map[string]string{}
	for _, c := range (&http.Response{Header: res.Header}).Cookies() {
		paths[c.Name] = c.Path
	}
	if paths["kaapi_rt"] != "/api/v1/auth" || paths["kaapi_at"] != "/" {
		t.Fatalf("cookie paths: %v", paths)
	}
}

func TestAnonymousAndBadTokens(t *testing.T) {
	s := apitest.New(t)
	if e := s.Do(http.MethodGet, "/v1/me", nil, nil).Error(t); e.Code != apperr.CodeUnauthenticated || e.Message != "Please sign in." {
		t.Fatalf("anonymous: %+v", e)
	}
	e := s.Do(http.MethodGet, "/v1/me", nil, map[string]string{"Authorization": "Bearer forged.token.value"}).Error(t)
	if e.Code != apperr.CodeUnauthenticated || e.Message == "Please sign in." {
		t.Fatalf("a present-but-bad token should say the session ended: %+v", e)
	}
}

func TestCrossSiteWritesAreRefused(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	evil := map[string]string{"Origin": "https://evil.example"}
	if e := s.Do(http.MethodPost, "/v1/auth/logout", nil, evil).Error(t); e.Code != apperr.CodeForbidden {
		t.Fatalf("cross-site logout: %s", e.Code)
	}
	ok := map[string]string{"Origin": apitest.TestConfig().AppOrigin}
	if res := s.Do(http.MethodPost, "/v1/auth/logout", nil, ok); res.Status != http.StatusNoContent {
		t.Fatalf("same-site logout: %d", res.Status)
	}
	if res := s.Do(http.MethodGet, "/v1/me", nil, nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("cookies should be cleared after logout: %d", res.Status)
	}
}
