// Package api is the HTTP layer: routes, request/response shapes and the gateway
// middleware. Domain packages stay free of net/http; handlers translate between JSON
// and domain calls and return errors for httpx.WriteError to render.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/httpx"
)

// API holds every dependency the handlers need.
type API struct {
	cfg       config.Config
	log       *slog.Logger
	pool      *pgxpool.Pool
	auth      *auth.Service
	catalog   *catalog.Service
	inventory *inventory.Service
	outbox    *outbox.Service
}

func New(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, now func() time.Time) *API {
	return &API{
		cfg:       cfg,
		log:       log,
		pool:      pool,
		auth:      auth.NewService(pool, cfg.JWESecret, now),
		catalog:   catalog.NewService(pool),
		inventory: inventory.NewService(pool, now),
		outbox:    outbox.NewService(pool),
	}
}

// Router builds the handler tree. Middleware order matters: the request ID comes first so
// every later log line and error carries it; Recover wraps everything that can panic.
func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(middleware.RealIP)
	r.Use(httpx.AccessLog(a.log))
	r.Use(httpx.Recover(a.log))
	r.Use(httpx.SecurityHeaders)
	r.Use(httpx.CORS(append([]string{a.cfg.AppOrigin}, a.cfg.NativeAppOrigins...)))
	r.Use(middleware.Timeout(a.cfg.RequestTimeout))
	r.NotFound(httpx.NotFound(a.log))
	r.MethodNotAllowed(httpx.MethodNotAllowed(a.log))

	h := func(fn httpx.HandlerFunc) http.HandlerFunc { return httpx.Handle(a.log, fn) }

	r.Get("/healthz", h(a.healthz))
	r.Get("/readyz", h(a.readyz))

	r.Route("/v1", func(r chi.Router) {
		// The gateway: identify the caller on every request, and refuse cross-site writes.
		r.Use(a.authenticate)
		r.Use(a.checkOrigin)

		r.Post("/auth/staff/login", h(a.staffLogin))
		r.Post("/auth/otp/request", h(a.otpRequest))
		r.Post("/auth/otp/verify", h(a.otpVerify))
		r.Post("/auth/refresh", h(a.refresh))
		r.Post("/auth/logout", h(a.logout))
		r.With(a.requireAuth).Get("/me", h(a.me))

		// Public: anyone can browse and hold a room.
		r.Get("/property", h(a.property))
		r.Get("/rooms", h(a.listRooms))
		r.Get("/rooms/{slug}", h(a.getRoom))
		r.Get("/rooms/{slug}/calendar", h(a.roomCalendar))
		r.Get("/availability", h(a.availability))
		r.Post("/holds", h(a.createHold))
		r.Delete("/holds/{id}", h(a.releaseHold))

		// Staff console. Each route names the permission it needs.
		r.Route("/staff", func(r chi.Router) {
			r.With(a.requirePermission(auth.PermOutboxRead)).Get("/outbox", h(a.listOutbox))
		})
	})
	return r
}
