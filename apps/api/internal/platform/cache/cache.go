// Package cache is a small in-process cache with a time-to-live, the hot store for results
// that are expensive enough to keep and are invalidated by the change feed. Consumers depend
// on their own small interface, so a Redis-backed store (needed once several API instances
// must share entries) can replace this without touching them.
package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

// TTL holds up to max entries for at most ttl each. The TTL is a safety net: entries are
// normally dropped as soon as the data behind them changes.
type TTL[K comparable, V any] struct {
	ttl time.Duration
	max int
	now func() time.Time

	mu      sync.Mutex
	entries map[K]entry[V]
}

func New[K comparable, V any](ttl time.Duration, max int) *TTL[K, V] {
	return &TTL[K, V]{ttl: ttl, max: max, now: time.Now, entries: map[K]entry[V]{}}
}

func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !c.now().Before(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTL[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		clear(c.entries) // bounded memory; the next requests repopulate what's in use
	}
	c.entries[key] = entry[V]{value: value, expires: c.now().Add(c.ttl)}
}

// Clear drops every entry.
func (c *TTL[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
}
