package httpx

import (
	"fmt"
	"net/http"
	"time"

	"kaapi/api/internal/platform/db"
)

const headerServerTiming = "Server-Timing"

// ServerTiming reports, on every response, the database work and total time the request took:
// `Server-Timing: db;dur=3.1;desc="4 queries", app;dur=5.2` (milliseconds). Browsers show it
// in the network panel, so latency in production is measured rather than guessed.
func ServerTiming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, t := db.WithTiming(r.Context())
		next.ServeHTTP(&timingWriter{ResponseWriter: w, timing: t, start: time.Now()}, r.WithContext(ctx))
	})
}

// timingWriter sets the header just before the status line goes out (after that, headers
// can't change), which is when the handler has done its database work.
type timingWriter struct {
	http.ResponseWriter
	timing  *db.Timing
	start   time.Time
	written bool
}

func (w *timingWriter) WriteHeader(code int) {
	if !w.written {
		w.written = true
		n, spent := w.timing.Totals()
		w.Header().Set(headerServerTiming, fmt.Sprintf(`db;dur=%.1f;desc="%d queries", app;dur=%.1f`,
			ms(spent), n, ms(time.Since(w.start))))
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *timingWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the real writer (flushing event streams).
func (w *timingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
