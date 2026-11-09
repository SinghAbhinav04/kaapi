// Package changes is the API's change feed: it listens on the 'data_changes' channel that
// database triggers write to (migration 0013) and hands each topic to whoever registered
// for it: cache invalidation inside the API, and open staff consoles through server-sent
// events. It's change data capture without extra infrastructure; the scale-up path is a
// logical-replication pipeline (or Redis pub/sub) behind the same Subscribe/OnChange calls.
package changes

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Topics, as the triggers send them.
const (
	Bookings      = "bookings"
	Inventory     = "inventory"
	Offers        = "offers"
	Enquiries     = "enquiries"
	Notifications = "notifications"
	// All is sent after (re)connecting: notices may have been missed while disconnected, so
	// everything is treated as changed.
	All = "*"
)

const (
	channel          = "data_changes"
	retryAfter       = 2 * time.Second
	subscriberBuffer = 16
)

type Feed struct {
	pool *pgxpool.Pool
	log  *slog.Logger

	mu        sync.Mutex
	callbacks []func(topic string)
	subs      map[chan string]struct{}
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Feed {
	return &Feed{pool: pool, log: log, subs: map[chan string]struct{}{}}
}

// OnChange registers a callback for every topic. Callbacks run on the listener's goroutine,
// so they must be quick (dropping a cache entry, not recomputing it).
func (f *Feed) OnChange(fn func(topic string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callbacks = append(f.callbacks, fn)
}

// Subscribe streams topics to a page; the returned func unsubscribes. A slow subscriber skips
// topics rather than holding up the others: it refetches everything on the next one anyway.
func (f *Feed) Subscribe() (<-chan string, func()) {
	ch := make(chan string, subscriberBuffer)
	f.mu.Lock()
	f.subs[ch] = struct{}{}
	f.mu.Unlock()
	return ch, func() {
		f.mu.Lock()
		delete(f.subs, ch)
		f.mu.Unlock()
	}
}

// Run listens until ctx ends, reconnecting if the database drops the connection. It holds one
// pooled connection for its lifetime; run it in the API process, where caches and pages are.
func (f *Feed) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := f.listenOnce(ctx); err != nil && ctx.Err() == nil {
			f.log.Warn("change feed reconnecting", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(retryAfter):
			}
		}
	}
}

func (f *Feed) listenOnce(ctx context.Context) error {
	conn, err := f.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	f.publish(All) // anything may have changed while we weren't listening
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		f.publish(n.Payload)
	}
}

// Publish hands a topic out as if the database had sent it; tests and the API's own writes
// can use it, though the triggers already cover every write.
func (f *Feed) Publish(topic string) { f.publish(topic) }

func (f *Feed) publish(topic string) {
	f.mu.Lock()
	callbacks := append([]func(string){}, f.callbacks...)
	for ch := range f.subs {
		select {
		case ch <- topic:
		default:
			// A slow subscriber's buffer is full. Dropping this topic would leave whatever it
			// covers stale, so swap the oldest queued one for "*": refetch everything.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- All:
			default:
			}
		}
	}
	f.mu.Unlock()
	for _, fn := range callbacks {
		fn(topic)
	}
}
