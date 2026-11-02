package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Chain spreads calls across many models and falls over when one fails.
//
// Models sit in tiers, tried in order: fast Gemini models, then Gemini Pro models, then
// Groq. Within a tier calls go round-robin, so each model's free-tier quota is used in turn
// instead of one model hitting its limit. A model that fails rests for a while (as long as
// the provider asks, for rate limits) and the next one is tried. Only when every model has
// failed does the caller fall back to the keyword rules.
type Chain struct {
	mu     sync.Mutex
	tiers  [][]*member
	cursor []int
	now    func() time.Time
}

type member struct {
	p         Provider
	gap       time.Duration // spacing between requests, from the model's requests-per-minute limit
	next      time.Time     // earliest moment the next request may go
	restUntil time.Time     // set after a failure
}

// Tier is one group of interchangeable models with the same per-model rate limit.
type Tier struct {
	Name      string
	Providers []Provider
	RPM       int // per model; 0 means no pacing
}

// maxWait caps how long a call waits for a paced slot in the current tier before moving on.
const maxWait = 20 * time.Second

func NewChain(tiers ...Tier) *Chain {
	c := &Chain{now: time.Now}
	c.setTiers(tiers)
	return c
}

// setTiers replaces the models (after discovery). Calls already running finish on the old set.
func (c *Chain) setTiers(tiers []Tier) {
	var built [][]*member
	for _, t := range tiers {
		if len(t.Providers) == 0 {
			continue
		}
		var gap time.Duration
		if t.RPM > 0 {
			gap = time.Minute / time.Duration(t.RPM)
		}
		ms := make([]*member, len(t.Providers))
		for i, p := range t.Providers {
			ms[i] = &member{p: p, gap: gap}
		}
		built = append(built, ms)
	}
	c.mu.Lock()
	c.tiers, c.cursor = built, make([]int, len(built))
	c.mu.Unlock()
}

// Empty reports whether the chain has no models at all.
func (c *Chain) Empty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tiers) == 0
}

// Models lists every model in calling order, for logs and the dashboard.
func (c *Chain) Models() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, t := range c.tiers {
		for _, m := range t {
			out = append(out, m.p.Name())
		}
	}
	return out
}

func (c *Chain) Name() string { return "chain(" + strings.Join(c.Models(), ",") + ")" }

func (c *Chain) Classify(ctx context.Context, message string, today time.Time) (Classification, error) {
	var out Classification
	err := c.call(ctx, func(p Provider) error {
		var err error
		out, err = p.Classify(ctx, message, today)
		return err
	})
	return out, err
}

func (c *Chain) Draft(ctx context.Context, in DraftInput) (string, error) {
	var out string
	err := c.call(ctx, func(p Provider) error {
		var err error
		out, err = p.Draft(ctx, in)
		return err
	})
	return out, err
}

func (c *Chain) call(ctx context.Context, fn func(Provider) error) error {
	tried := map[*member]bool{}
	var errs []error
	for {
		m, wait := c.pick(tried)
		if m == nil && wait == 0 {
			break
		}
		if m == nil {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
			continue
		}
		tried[m] = true
		err := fn(m.p)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.rest(m, err)
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return fmt.Errorf("%w: every model is resting after recent failures", ErrUnavailable)
	}
	return fmt.Errorf("%w: every model failed: %w", ErrUnavailable, errors.Join(errs...))
}

// pick returns the next model to try, or how long to wait for one in the current tier.
// A tier counts as exhausted only when all its models have failed or are resting; a model
// that is merely paced is waited for, so work stays on the preferred tier.
func (c *Chain) pick(tried map[*member]bool) (*member, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for ti, tier := range c.tiers {
		var soonest time.Duration
		for i := range tier {
			idx := (c.cursor[ti] + i) % len(tier)
			m := tier[idx]
			if tried[m] || now.Before(m.restUntil) {
				continue
			}
			if wait := m.next.Sub(now); wait > 0 {
				if soonest == 0 || wait < soonest {
					soonest = wait
				}
				continue
			}
			m.next = now.Add(m.gap)
			c.cursor[ti] = (idx + 1) % len(tier)
			return m, 0
		}
		if soonest > 0 && soonest <= maxWait {
			return nil, soonest
		}
	}
	return nil, 0
}

func (c *Chain) rest(m *member, err error) {
	d := restUnavailable
	var pe *ProviderError
	if errors.As(err, &pe) {
		d = pe.rest()
	}
	if d == 0 {
		return
	}
	c.mu.Lock()
	m.restUntil = c.now().Add(d)
	c.mu.Unlock()
}
