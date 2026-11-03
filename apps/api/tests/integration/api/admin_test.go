package api_test

import (
	"net/http"
	"testing"

	"kaapi/api/internal/api"
	"kaapi/api/internal/seed"
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
