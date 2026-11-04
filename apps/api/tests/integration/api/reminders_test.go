package api_test

import (
	"testing"

	"kaapi/api/tests/apitest"
)

// A guest arriving soon gets one pre-arrival email and, on the day they leave, one check-out
// email; running the job again (or in another worker) sends nothing more.
func TestRemindersGoOutExactlyOnce(t *testing.T) {
	s := apitest.New(t) // "today" is 2 Nov 2026, 14:30 IST
	var arriving, leaving string
	if err := s.Pool.QueryRow(s.Ctx, `SELECT ref FROM bookings WHERE status = 'CONFIRMED' ORDER BY ref LIMIT 1`).Scan(&arriving); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(s.Ctx, `SELECT ref FROM bookings WHERE status = 'CONFIRMED' AND ref <> $1 ORDER BY ref LIMIT 1`, arriving).Scan(&leaving); err != nil {
		t.Fatal(err)
	}
	for ref, stay := range map[string][2]string{arriving: {"2026-11-03", "2026-11-05"}, leaving: {"2026-10-31", "2026-11-02"}} {
		if _, err := s.Pool.Exec(s.Ctx, `UPDATE bookings SET check_in = $2, check_out = $3 WHERE ref = $1`, ref, stay[0], stay[1]); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.App.Bookings.QueueReminders(s.Ctx); err != nil {
		t.Fatal(err)
	}
	count := func(ref, kind string) int {
		var n int
		if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) FROM outbox_messages o JOIN bookings b ON b.id = o.booking_id
			WHERE b.ref = $1 AND o.kind = $2`, ref, kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(arriving, "CHECKIN_REMINDER") != 1 || count(leaving, "CHECKOUT_REMINDER") != 1 {
		t.Fatalf("checkin=%d checkout=%d, want 1 each", count(arriving, "CHECKIN_REMINDER"), count(leaving, "CHECKOUT_REMINDER"))
	}
	again, err := s.App.Bookings.QueueReminders(s.Ctx)
	if err != nil || again != 0 {
		t.Fatalf("second run queued %d (err %v), want 0", again, err)
	}
}
