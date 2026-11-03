package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"kaapi/api/internal/platform/ratelimit"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestFixedWindowAllowsTheLimitThenResets(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 3, 10, 0, 10, 0, time.UTC)}
	l := ratelimit.New(nil, []byte("test-secret"), c.now)
	rule := ratelimit.Rule{Name: "holds", Limit: 3, Window: time.Minute}
	who := l.Subject("ip", "203.0.113.7")
	for i := 1; i <= 3; i++ {
		if d, _ := l.Allow(context.Background(), rule, who); !d.Allowed || d.Remaining != 3-i {
			t.Fatalf("request %d: %+v", i, d)
		}
	}
	d, _ := l.Allow(context.Background(), rule, who)
	if d.Allowed || d.RetryAfter != 50*time.Second {
		t.Fatalf("4th request must wait for the window to reset: %+v", d)
	}
	if other, _ := l.Allow(context.Background(), rule, l.Subject("ip", "203.0.113.8")); !other.Allowed {
		t.Fatal("another address has its own counter")
	}
	c.t = c.t.Add(50 * time.Second) // the next window
	if d, _ := l.Allow(context.Background(), rule, who); !d.Allowed {
		t.Fatalf("new window: %+v", d)
	}
}

// Rules have different window lengths; a sweep triggered by a short rule must not wipe a long
// rule's counter early (which would reset its limit).
func TestLongWindowsSurviveSweepsFromShortOnes(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}
	l := ratelimit.New(nil, []byte("test-secret"), c.now)
	long := ratelimit.Rule{Name: "login", Limit: 2, Window: 15 * time.Minute}
	short := ratelimit.Rule{Name: "public", Limit: 100, Window: time.Minute}
	who := l.Subject("ip", "198.51.100.1")
	l.Allow(context.Background(), long, who)
	l.Allow(context.Background(), long, who)
	c.t = c.t.Add(5 * time.Minute) // past the sweep interval; the short rule sweeps
	l.Allow(context.Background(), short, who)
	if d, _ := l.Allow(context.Background(), long, who); d.Allowed {
		t.Fatal("the 15-minute counter was reset by a sweep")
	}
}

func TestSubjectsAreOpaque(t *testing.T) {
	l := ratelimit.New(nil, []byte("test-secret"), nil)
	s := l.Subject("ip", "203.0.113.7")
	if s == "ip:203.0.113.7" || len(s) < 10 || s[:3] != "ip:" {
		t.Fatalf("subject should be a keyed hash, not the raw address: %q", s)
	}
	if s != l.Subject("ip", "203.0.113.7") || s == l.Subject("email", "203.0.113.7") {
		t.Fatal("subjects must be stable and separated by kind")
	}
}
