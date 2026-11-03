package api_test

import (
	"net/http"
	"testing"

	"kaapi/api/internal/seed"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

func TestEnvironmentAdministratorCanSignIn(t *testing.T) {
	s := apitest.New(t)
	account := seed.Account{Email: "Owner.Admin@Example.test", Password: "first-Pass(1)"}
	if err := seed.EnsureAdmin(s.Ctx, s.Pool, account); err != nil {
		t.Fatal(err)
	}

	var out api.AuthResponse
	res := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "owner.admin@example.test", Password: account.Password}, nil)
	res.JSON(t, &out)
	if res.Status != http.StatusOK || out.User.Role != "ADMIN" {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	// The administrator sees every lane.
	if r := s.Do(http.MethodGet, "/v1/staff/enquiries?lane=GM", nil, nil); r.Status != http.StatusOK {
		t.Fatalf("admin inbox: %d", r.Status)
	}

	// Running it again resets the password and signs the old session out.
	account.Password = "second-Pass(2)"
	if err := seed.EnsureAdmin(s.Ctx, s.Pool, account); err != nil {
		t.Fatal(err)
	}
	if r := s.Do(http.MethodPost, "/v1/auth/refresh", nil, nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("old session should be revoked: %d", r.Status)
	}
	fresh := s.NewSession()
	if r := fresh.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: account.Email, Password: "first-Pass(1)"}, nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("old password still works: %d", r.Status)
	}
	if r := fresh.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: account.Email, Password: account.Password}, nil); r.Status != http.StatusOK {
		t.Fatalf("new password: %d", r.Status)
	}

	for _, bad := range []seed.Account{{Email: "not-an-email", Password: "long-enough-1"}, {Email: "a@example.test", Password: "short"}} {
		if err := seed.EnsureAdmin(s.Ctx, s.Pool, bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

// Only the administrator can run the retention sweep by hand; it reports what it changed.
func TestRetentionRunIsAdminOnly(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "sales@kaapiestate.test", nil)
	if r := s.Do(http.MethodPost, "/v1/staff/retention/run", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("sales: %d, want 403", r.Status)
	}

	admin := s.NewSession()
	staffLogin(t, admin, "admin@kaapiestate.test", nil)
	var out api.RetentionResponse
	res := admin.Do(http.MethodPost, "/v1/staff/retention/run", nil, nil)
	res.JSON(t, &out)
	if res.Status != http.StatusOK {
		t.Fatalf("admin: %d %s", res.Status, res.Body)
	}
}

// Analytics is for the GM and administrator. Its revenue matches the bookings table, and
// the daily series has one row per day of the range.
func TestAnalyticsReport(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "sales@kaapiestate.test", nil)
	if r := s.Do(http.MethodGet, "/v1/staff/analytics", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("sales: %d, want 403", r.Status)
	}

	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	var out api.AnalyticsResponse
	res := gm.Do(http.MethodGet, "/v1/staff/analytics?from=2026-08-01&to=2026-11-02", nil, nil)
	res.JSON(t, &out)
	if res.Status != http.StatusOK {
		t.Fatalf("gm: %d %s", res.Status, res.Body)
	}
	var want int64
	if err := s.Pool.QueryRow(s.Ctx, `SELECT COALESCE(sum(total_paise), 0) FROM bookings WHERE status = 'CONFIRMED'
		AND created_at >= '2026-08-01T00:00:00+05:30' AND created_at < '2026-11-03T00:00:00+05:30'`).Scan(&want); err != nil {
		t.Fatal(err)
	}
	if out.KPIs.RevenuePaise != want || want == 0 {
		t.Errorf("revenue %d, want %d", out.KPIs.RevenuePaise, want)
	}
	if len(out.Daily) != 94 { // 1 Aug to 2 Nov inclusive
		t.Errorf("daily rows = %d, want 94", len(out.Daily))
	}
	var sum int64
	for _, d := range out.Daily {
		sum += d.RevenuePaise
	}
	if sum != want {
		t.Errorf("daily revenue adds to %d, want %d", sum, want)
	}
	if len(out.OnTheBooks) != 6 {
		t.Errorf("on the books has %d months, want 6", len(out.OnTheBooks))
	}

	if r := gm.Do(http.MethodGet, "/v1/staff/analytics?from=2026-11-02&to=2026-10-01", nil, nil); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("backwards range: %d, want 422", r.Status)
	}
}
