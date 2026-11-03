package httpx

import (
	"log/slog"
	"net/http"
	"slices"
	"time"
)

// Response headers set on every request.
const (
	headerCSP            = "Content-Security-Policy"
	headerNoSniff        = "X-Content-Type-Options"
	headerFrameOptions   = "X-Frame-Options"
	headerReferrer       = "Referrer-Policy"
	headerCacheControl   = "Cache-Control"
	headerVary           = "Vary"
	headerAllowOrigin    = "Access-Control-Allow-Origin"
	headerAllowMethods   = "Access-Control-Allow-Methods"
	headerAllowHeaders   = "Access-Control-Allow-Headers"
	headerAllowCreds     = "Access-Control-Allow-Credentials"
	headerMaxAge         = "Access-Control-Max-Age"
	corsAllowedMethods   = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders   = "Authorization, Content-Type, Idempotency-Key, X-Hold-Token, X-Request-Id"
	corsPreflightSeconds = "600"
)

// SecurityHeaders sets conservative defaults. The API serves JSON only, so its CSP forbids everything.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set(headerCSP, "default-src 'none'; frame-ancestors 'none'")
		h.Set(headerNoSniff, "nosniff")
		h.Set(headerFrameOptions, "DENY")
		h.Set(headerReferrer, "no-referrer")
		h.Set(headerCacheControl, "no-store")
		next.ServeHTTP(w, r)
	})
}

// CORS allows the listed origins only. The web app calls the API through its own origin
// (a rewrite), so this exists for the Capacitor shell and local development.
func CORS(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(origins, origin) {
				h := w.Header()
				h.Set(headerAllowOrigin, origin)
				h.Set(headerAllowCreds, "true")
				h.Add(headerVary, "Origin")
				if r.Method == http.MethodOptions {
					h.Set(headerAllowMethods, corsAllowedMethods)
					h.Set(headerAllowHeaders, corsAllowedHeaders)
					h.Set(headerMaxAge, corsPreflightSeconds)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder remembers the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }

// Unwrap lets http.ResponseController reach the real writer, so streams can flush.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// AccessLog writes one structured line per request.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			log.InfoContext(r.Context(), "http", "request_id", RequestIDFrom(r.Context()), "method", r.Method,
				"path", r.URL.Path, "status", rec.status, "bytes", rec.bytes, "ms", time.Since(start).Milliseconds())
		})
	}
}
