package ratelimit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Limiter struct {
	pool *pgxpool.Pool
	key  []byte
	now  func() time.Time

	mu        sync.Mutex
	counts    map[string]count
	lastSweep time.Time
}

type count struct {
	window time.Time
	ends   time.Time // the counter is dropped after this; rules have different window lengths
	n      int
}

// New counts shared rules in pool (nil: everything in memory, e.g. in unit tests). The secret
// keys the HMAC that turns an IP or email into a counter key.
func New(pool *pgxpool.Pool, secret []byte, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(keyDomain))
	return &Limiter{pool: pool, key: m.Sum(nil), now: now, counts: map[string]count{}}
}

// Subject turns an identifier (an IP, an email, a user id) into an opaque counter key.
func (l *Limiter) Subject(kind, value string) string {
	m := hmac.New(sha256.New, l.key)
	m.Write([]byte(kind + "|" + value))
	return kind + ":" + base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}

// Allow counts one request by subject against rule.
func (l *Limiter) Allow(ctx context.Context, rule Rule, subject string) (Decision, error) {
	now := l.now()
	window := now.Truncate(rule.Window)
	key := rule.Name + "/" + subject
	var n int
	if rule.Shared && l.pool != nil {
		// One upsert per request: the row is the window's counter, shared by every instance.
		// The retention sweep deletes old windows.
		if err := l.pool.QueryRow(ctx, `INSERT INTO rate_limit_buckets (key, window_start, count) VALUES ($1, $2, 1)
			ON CONFLICT (key, window_start) DO UPDATE SET count = rate_limit_buckets.count + 1
			RETURNING count`, key, window).Scan(&n); err != nil {
			return Decision{Allowed: true}, err // fail open: an outage must not lock everyone out
		}
	} else {
		n = l.incr(key, window, window.Add(rule.Window), now)
	}
	d := Decision{Allowed: n <= rule.Limit, Remaining: max(rule.Limit-n, 0)}
	if !d.Allowed {
		d.RetryAfter = window.Add(rule.Window).Sub(now)
	}
	return d, nil
}

func (l *Limiter) incr(key string, window, ends, now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) > memorySweepEvery {
		for k, c := range l.counts {
			if !c.ends.After(now) {
				delete(l.counts, k)
			}
		}
		l.lastSweep = now
	}
	c := l.counts[key]
	if !c.window.Equal(window) {
		c = count{window: window, ends: ends}
	}
	c.n++
	l.counts[key] = c
	return c.n
}
