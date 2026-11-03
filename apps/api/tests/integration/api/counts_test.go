package api_test

import (
	"net/http"
	"testing"

	"kaapi/api/internal/domain"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// The sidebar's counts must equal what the inbox lists, lane by lane, for each role, and show
// the reply time only to people who can see the dashboard.
func TestInboxCountsMatchTheInboxPerRole(t *testing.T) {
	s := apitest.New(t)
	if _, err := s.App.ProcessEnquiries(s.Ctx); err != nil { // lanes are assigned when the AI (or keyword rules) read them
		t.Fatal(err)
	}

	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	var all api.EnquiriesResponse
	gm.Do(http.MethodGet, "/v1/staff/enquiries?limit=300", nil, nil).JSON(t, &all)
	listed := map[domain.Lane]int{}
	for _, e := range all.Enquiries {
		listed[e.Lane]++
	}
	if listed[domain.LaneReservations] == 0 || listed[domain.LaneSales] == 0 {
		t.Fatalf("the seed should fill the lanes once read: %v", listed)
	}
	var counts api.InboxCountsResponse
	gm.Do(http.MethodGet, "/v1/staff/inbox/counts", nil, nil).JSON(t, &counts)
	if len(counts.Lanes) != len(domain.Lanes) || counts.ReplyTime == nil || counts.ReplyTime.TargetMinutes == 0 {
		t.Fatalf("GM sees every lane and the reply time: %+v", counts)
	}
	for _, c := range counts.Lanes {
		if c.Count != listed[c.Lane] {
			t.Errorf("%s: counted %d, the inbox lists %d", c.Lane, c.Count, listed[c.Lane])
		}
	}

	res := s.NewSession()
	staffLogin(t, res, "reservations@kaapiestate.test", nil)
	var own api.InboxCountsResponse
	res.Do(http.MethodGet, "/v1/staff/inbox/counts", nil, nil).JSON(t, &own)
	if len(own.Lanes) != 1 || own.Lanes[0].Lane != domain.LaneReservations || own.Lanes[0].Count != listed[domain.LaneReservations] {
		t.Fatalf("reservations see only their lane: %+v", own.Lanes)
	}

	// A role without the dashboard gets lane counts but no reply-time figure.
	if _, err := s.Pool.Exec(s.Ctx, `DELETE FROM role_permissions WHERE permission = 'dashboard:read'
		AND role_id = (SELECT id FROM roles WHERE key = 'SALES')`); err != nil {
		t.Fatal(err)
	}
	sales := s.NewSession()
	staffLogin(t, sales, "sales@kaapiestate.test", nil)
	var theirs api.InboxCountsResponse
	sales.Do(http.MethodGet, "/v1/staff/inbox/counts", nil, nil).JSON(t, &theirs)
	if len(theirs.Lanes) != 1 || theirs.Lanes[0].Lane != domain.LaneSales || theirs.Lanes[0].Count != listed[domain.LaneSales] || theirs.ReplyTime != nil {
		t.Fatalf("sales without the dashboard: %+v", theirs)
	}
}
