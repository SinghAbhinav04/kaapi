package api_test

import (
	"testing"

	"kaapi/api/tests/apitest"
)

// Each reminder goes out exactly once: running the job again (or in another worker) sends
// nothing more. "Today" in tests is 2 Nov 2026, 14:30 IST: after the check-in-day reminder
// starts (8 am) and after check-out time (11 am), so the check-out-day one is not due.
func TestRemindersGoOutExactlyOnce(t *testing.T) {
	s := apitest.New(t)
	rows, err := s.Pool.Query(s.Ctx, `SELECT ref FROM bookings WHERE status = 'CONFIRMED' ORDER BY ref LIMIT 4`)
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for rows.Next() {
		var r string
		_ = rows.Scan(&r)
		refs = append(refs, r)
	}
	soon, today, leavingSoon, leavingToday := refs[0], refs[1], refs[2], refs[3]
	stays := map[string][2]string{
		soon:         {"2026-11-04", "2026-11-06"}, // arriving in 2 days
		today:        {"2026-11-02", "2026-11-04"}, // arriving today
		leavingSoon:  {"2026-10-31", "2026-11-04"}, // staying, leaving in 2 days
		leavingToday: {"2026-10-30", "2026-11-02"}, // leaving today, but it's past 11 am
	}
	// Park every other seeded booking far away so only these four can be due.
	if _, err := s.Pool.Exec(s.Ctx, `UPDATE bookings SET check_in = '2027-06-01', check_out = '2027-06-03' WHERE NOT (ref = ANY($1))`, refs); err != nil {
		t.Fatal(err)
	}
	for ref, st := range stays {
		if _, err := s.Pool.Exec(s.Ctx, `UPDATE bookings SET check_in = $2, check_out = $3 WHERE ref = $1`, ref, st[0], st[1]); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.App.Bookings.QueueReminders(s.Ctx)
	if err != nil {
		t.Fatal(err)
	}
	kinds := func(ref string) []string {
		rows, err := s.Pool.Query(s.Ctx, `SELECT r.kind FROM booking_reminders r JOIN bookings b ON b.id = r.booking_id WHERE b.ref = $1 ORDER BY 1`, ref)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for rows.Next() {
			var k string
			_ = rows.Scan(&k)
			out = append(out, k)
		}
		return out
	}
	want := map[string]string{soon: "CHECKIN_SOON", today: "CHECKIN_TODAY", leavingSoon: "CHECKOUT_SOON"}
	for ref, k := range want {
		if got := kinds(ref); len(got) != 1 || got[0] != k {
			t.Errorf("%s: reminders %v, want [%s]", ref, got, k)
		}
	}
	if got := kinds(leavingToday); len(got) != 0 {
		t.Errorf("past check-out time, yet reminded: %v", got)
	}
	if n != 3 {
		t.Errorf("queued %d, want 3", n)
	}
	if again, err := s.App.Bookings.QueueReminders(s.Ctx); err != nil || again != 0 {
		t.Fatalf("second run queued %d (err %v), want 0", again, err)
	}
}
