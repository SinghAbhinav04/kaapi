package db

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Timing adds up the database work done for one request: how many queries, and how long
// they took. The Server-Timing header reports it, so production numbers are visible in the
// browser's network panel rather than estimated.
type Timing struct {
	mu      sync.Mutex
	queries int
	spent   time.Duration
}

// Totals returns the queries run and the time spent in them so far.
func (t *Timing) Totals() (int, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.queries, t.spent
}

type timingKey struct{}
type queryStartKey struct{}

// WithTiming starts counting database work done with ctx (and contexts derived from it).
func WithTiming(ctx context.Context) (context.Context, *Timing) {
	t := &Timing{}
	return context.WithValue(ctx, timingKey{}, t), t
}

// Tracer is the query tracer every pool should use, so Server-Timing counts its queries;
// Connect sets it, and pools built elsewhere (tests) set it themselves.
func Tracer() pgx.QueryTracer { return queryTracer{} }

// queryTracer feeds Timing; queries outside a timed request cost one context lookup.
type queryTracer struct{}

func (queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	if ctx.Value(timingKey{}) == nil {
		return ctx
	}
	return context.WithValue(ctx, queryStartKey{}, time.Now())
}

func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	t, _ := ctx.Value(timingKey{}).(*Timing)
	start, ok := ctx.Value(queryStartKey{}).(time.Time)
	if t == nil || !ok {
		return
	}
	t.mu.Lock()
	t.queries++
	t.spent += time.Since(start)
	t.mu.Unlock()
}
