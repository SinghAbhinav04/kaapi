// Package ratelimit counts requests per caller in fixed windows. Two stores back it: memory,
// for high-volume reads where a per-process count is enough, and the rate_limit_buckets table
// for actions that must hold across every API instance (sign-in attempts, holds that lock
// inventory, bookings, enquiries). Keys are HMACs, so no raw IP or email is stored.
package ratelimit

import "time"

// Rule is one limit: at most Limit requests per Window for each caller.
type Rule struct {
	Name   string        // part of the key, so rules never share counters
	Limit  int           // requests allowed in one window
	Window time.Duration // fixed window; it resets on the boundary
	Shared bool          // counted in the database (every instance) rather than in this process
}

// Decision is the answer for one request.
type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration // until the window resets; zero when allowed
}

const (
	// memorySweepEvery bounds how often the in-memory store drops finished windows.
	memorySweepEvery = time.Minute
	keyDomain        = "kaapi/rate-limit"
)
