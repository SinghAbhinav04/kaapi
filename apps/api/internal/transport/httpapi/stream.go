package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/auth"
)

const (
	inboxChannel     = "inbox_changes" // see migrations/0006_inbox_notify.sql
	streamHeartbeat  = 25 * time.Second
	streamRetryMS    = 3000
	listenRetry      = 2 * time.Second
	subscriberBuffer = 16
	streamPathSuffix = "/stream"
)

// hub holds one LISTEN connection while any inbox page is open and fans each change out
// to all of them, so a hundred open tabs cost one database connection, and none cost none.
type hub struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	mu   sync.Mutex
	subs map[chan string]struct{}
	stop context.CancelFunc // ends the listener; nil while nobody is subscribed
}

func newHub(pool *pgxpool.Pool, log *slog.Logger) *hub {
	return &hub{pool: pool, log: log, subs: map[chan string]struct{}{}}
}

func (h *hub) subscribe() (<-chan string, func()) {
	ch := make(chan string, subscriberBuffer)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	if h.stop == nil {
		ctx, cancel := context.WithCancel(context.Background())
		h.stop = cancel
		go h.listen(ctx)
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		if len(h.subs) == 0 && h.stop != nil { // last page closed: give the connection back
			h.stop()
			h.stop = nil
		}
		h.mu.Unlock()
	}
}

// listen keeps a LISTEN connection open until ctx ends, reconnecting if the database
// drops it. A missed change only delays a page until its next refresh.
func (h *hub) listen(ctx context.Context) {
	for ctx.Err() == nil {
		if err := h.listenOnce(ctx); err != nil && ctx.Err() == nil {
			h.log.Warn("inbox listener reconnecting", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(listenRetry):
			}
		}
	}
}

func (h *hub) listenOnce(ctx context.Context) error {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+inboxChannel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		h.mu.Lock()
		for ch := range h.subs {
			select {
			case ch <- n.Payload:
			default: // a slow page skips a beat; it refetches everything on the next one anyway
			}
		}
		h.mu.Unlock()
	}
}

// inboxStream: GET /v1/staff/inbox/stream — server-sent events, one "inbox" event (data:
// the enquiry ref) whenever an enquiry or its draft changes. The page refetches with its
// own permissions, so the stream carries only refs.
func (a *API) inboxStream(w http.ResponseWriter, r *http.Request) error {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's write timeout would cut a long-lived stream
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // proxies must pass events through, not buffer them
	w.WriteHeader(http.StatusOK)

	events, stop := a.hub.subscribe()
	defer stop()
	if _, err := fmt.Fprintf(w, "retry: %d\n: connected\n\n", streamRetryMS); err != nil {
		return nil
	}
	_ = rc.Flush()
	beat := time.NewTicker(streamHeartbeat)
	defer beat.Stop()
	for {
		var err error
		select {
		case <-r.Context().Done():
			return nil
		case ref := <-events:
			_, err = fmt.Fprintf(w, "event: inbox\ndata: %s\n\n", ref)
		case <-beat.C:
			_, err = fmt.Fprint(w, ": ping\n\n") // keeps proxies from closing an idle stream
		}
		if err != nil || rc.Flush() != nil {
			return nil
		}
	}
}

// changesStream: GET /v1/staff/changes/stream: server-sent events from the change feed, one
// "change" event per topic (bookings, inventory, offers, enquiries, or * after the API
// reconnected to the database). Staff consoles refetch what that topic affects instead of
// polling (the sidebar's lane counts too, so anyone with an inbox gets it). Only topic names
// are sent; every refetch goes through the caller's permissions.
func (a *API) changesStream(w http.ResponseWriter, r *http.Request) error {
	if !a.can(r, auth.PermRatesRead) && !a.can(r, auth.PermAnalyticsRead) && !a.can(r, auth.PermPromosWrite) && !a.can(r, auth.PermEnquiriesRead) {
		return auth.ErrForbidden
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's write timeout would cut a long-lived stream
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	topics, stop := a.changes.Subscribe()
	defer stop()
	if _, err := fmt.Fprintf(w, "retry: %d\n: connected\n\n", streamRetryMS); err != nil {
		return nil
	}
	_ = rc.Flush()
	beat := time.NewTicker(streamHeartbeat)
	defer beat.Stop()
	for {
		var err error
		select {
		case <-r.Context().Done():
			return nil
		case topic := <-topics:
			_, err = fmt.Fprintf(w, "event: change\ndata: %s\n\n", topic)
		case <-beat.C:
			_, err = fmt.Fprint(w, ": ping\n\n")
		}
		if err != nil || rc.Flush() != nil {
			return nil
		}
	}
}

// timeoutExceptStreams applies the request timeout to everything but event streams.
func timeoutExceptStreams(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		timed := middleware.Timeout(d)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, streamPathSuffix) {
				next.ServeHTTP(w, r)
				return
			}
			timed.ServeHTTP(w, r)
		})
	}
}
