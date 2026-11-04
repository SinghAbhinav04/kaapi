package api_test

import (
	"net/http"
	"slices"
	"testing"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// The permissions table and the code's closed set must agree, or a grant could name a
// permission no route checks (or a route could check one nobody can be granted).
func TestPermissionCatalogueMatchesCode(t *testing.T) {
	s := apitest.New(t)
	rows, err := s.Pool.Query(s.Ctx, `SELECT key FROM permissions ORDER BY key`)
	if err != nil {
		t.Fatal(err)
	}
	var inDB []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		inDB = append(inDB, k)
	}
	var inCode []string
	for _, p := range auth.KnownPermissions() {
		inCode = append(inCode, string(p))
	}
	slices.Sort(inCode)
	if !slices.Equal(inDB, inCode) {
		t.Fatalf("permissions table %v\ncode %v", inDB, inCode)
	}
}

func TestAdminManagesRolesAndPeople(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "admin@kaapiestate.test", nil)

	// A role for the front desk: the inbox and bookings, nothing else.
	var created api.RoleResponse
	r := s.Do(http.MethodPost, "/v1/staff/roles", api.RoleRequest{Name: "Front desk", Description: "Arrivals and walk-ins",
		Team: "RESERVATIONS", Permissions: []auth.Permission{auth.PermEnquiriesRead, auth.PermBookingsRead}}, nil)
	r.JSON(t, &created)
	if r.Status != http.StatusCreated || created.Role.Key != "FRONT_DESK" || len(created.Role.Permissions) != 2 {
		t.Fatalf("create role: %d %s", r.Status, r.Body)
	}
	roleID := created.Role.ID.String()

	var member api.MemberResponse
	r = s.Do(http.MethodPost, "/v1/staff/users", api.CreateMemberRequest{Email: "Desk@Kaapiestate.test", Name: "Ravi Desk",
		RoleID: created.Role.ID, Password: "front-desk-pass-1"}, nil)
	r.JSON(t, &member)
	if r.Status != http.StatusCreated || member.Member.Email != "desk@kaapiestate.test" || member.Member.Team != "RESERVATIONS" {
		t.Fatalf("create member: %d %s", r.Status, r.Body)
	}
	if r := s.Do(http.MethodPost, "/v1/staff/users", api.CreateMemberRequest{Email: "desk@kaapiestate.test", Name: "Again",
		RoleID: created.Role.ID, Password: "front-desk-pass-1"}, nil); r.Status != http.StatusConflict {
		t.Fatalf("duplicate email: %d", r.Status)
	}

	// The new person sees exactly what the role grants.
	desk := s.NewSession()
	me := staffSignIn(t, desk, "desk@kaapiestate.test", "front-desk-pass-1", nil)
	if len(me.User.Permissions) != 2 {
		t.Fatalf("permissions: %v", me.User.Permissions)
	}
	if r := desk.Do(http.MethodGet, "/v1/staff/bookings", nil, nil); r.Status != http.StatusOK {
		t.Fatalf("bookings: %d", r.Status)
	}
	if r := desk.Do(http.MethodGet, "/v1/staff/rates?from=2026-11-10&to=2026-11-12", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("rates before grant: %d, want 403", r.Status)
	}
	if r := desk.Do(http.MethodGet, "/v1/staff/access", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("access console: %d, want 403", r.Status)
	}

	// Granting rates applies on the person's next request, no sign-out needed.
	r = s.Do(http.MethodPut, "/v1/staff/roles/"+roleID, api.RoleRequest{Name: "Front desk", Team: "RESERVATIONS",
		Permissions: []auth.Permission{auth.PermEnquiriesRead, auth.PermBookingsRead, auth.PermRatesRead}}, nil)
	if r.Status != http.StatusOK {
		t.Fatalf("update role: %d %s", r.Status, r.Body)
	}
	if r := desk.Do(http.MethodGet, "/v1/staff/rates?from=2026-11-10&to=2026-11-12", nil, nil); r.Status != http.StatusOK {
		t.Fatalf("rates after grant: %d %s", r.Status, r.Body)
	}

	// A role people still have can't be deleted; the system roles can't be edited.
	if r := s.Do(http.MethodDelete, "/v1/staff/roles/"+roleID, nil, nil); r.Status != http.StatusConflict {
		t.Fatalf("delete role in use: %d", r.Status)
	}
	var overview api.AccessResponse
	s.Do(http.MethodGet, "/v1/staff/access", nil, nil).JSON(t, &overview)
	for _, role := range overview.Roles {
		if role.Key == "ADMIN" {
			if r := s.Do(http.MethodPut, "/v1/staff/roles/"+role.ID.String(), api.RoleRequest{Name: "Boss", Team: "GM",
				Permissions: []auth.Permission{}}, nil); r.Status != http.StatusConflict {
				t.Fatalf("edit Administrator: %d", r.Status)
			}
		}
	}

	// Deactivating cuts access at once, even with a live access token.
	off := false
	if r := s.Do(http.MethodPatch, "/v1/staff/users/"+member.Member.ID.String(), api.UpdateMemberRequest{Active: &off}, nil); r.Status != http.StatusOK {
		t.Fatalf("deactivate: %d %s", r.Status, r.Body)
	}
	if r := desk.Do(http.MethodGet, "/v1/staff/bookings", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("bookings after deactivation: %d, want 403", r.Status)
	}

	// The administrator can't lock themselves out.
	var self api.AuthResponse
	s.Do(http.MethodGet, "/v1/me", nil, nil).JSON(t, &self)
	if r := s.Do(http.MethodPatch, "/v1/staff/users/"+self.User.ID.String(), api.UpdateMemberRequest{Active: &off}, nil); r.Status != http.StatusConflict {
		t.Fatalf("self-deactivate: %d, want 409", r.Status)
	}

	// Only people with users:manage reach the console.
	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	if r := gm.Do(http.MethodGet, "/v1/staff/access", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("gm on access console: %d, want 403", r.Status)
	}
}

// The built-in roles grant exactly what the code granted before roles moved into the database.
func TestBuiltInRoleGrants(t *testing.T) {
	s := apitest.New(t)
	cases := []struct {
		role domain.Role
		perm auth.Permission
		want bool
	}{
		{domain.RoleGuest, auth.PermOwnBookingsRead, true},
		{domain.RoleGuest, auth.PermOutboxRead, false},
		{domain.RoleGuest, auth.PermRatesWrite, false},
		{domain.RoleSales, auth.PermEnquirySend, true},
		{domain.RoleSales, auth.PermRatesWrite, false},
		{domain.RoleReservations, auth.PermRatesWrite, true},
		{domain.RoleReservations, auth.PermPromosWrite, false},
		{domain.RoleReservations, auth.PermEnquiriesAll, false},
		{domain.RoleGM, auth.PermEnquiriesAll, true},
		{domain.RoleGM, auth.PermPromosWrite, true},
	}

	for _, tc := range cases {
		var has bool
		if err := s.Pool.QueryRow(s.Ctx, `SELECT EXISTS (SELECT 1 FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
			WHERE r.key = $1 AND rp.permission = $2)`, string(tc.role), string(tc.perm)).Scan(&has); err != nil {
			t.Fatal(err)
		}
		if has != tc.want {
			t.Errorf("%s / %s: %v, want %v", tc.role, tc.perm, has, tc.want)
		}
	}
}
