// Package api is the HTTP layer: routes, request/response shapes and the gateway
// middleware. Domain packages stay free of net/http; handlers translate between JSON
// and domain calls and return errors for httpx.WriteError to render.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/admin"
	"kaapi/api/internal/ai"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/enquiry"
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
	admin     *admin.Service
	auth      *auth.Service
	bookings  *booking.Service
	catalog   *catalog.Service
	enquiries *enquiry.Service
	inventory *inventory.Service
	outbox    *outbox.Service
}

func New(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, now func() time.Time) *API {
	inv := inventory.NewService(pool, now)
	var provider ai.Provider // nil: keyword rules and template drafts only
	if cfg.GeminiAPIKey != "" {
		provider = ai.NewGemini(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.GeminiBaseURL, cfg.GeminiRPM)
	}
	return &API{
		cfg:       cfg,
		log:       log,
		pool:      pool,
		admin:     admin.NewService(pool),
		auth:      auth.NewService(pool, cfg.JWESecret, now),
		bookings:  booking.NewService(pool, now),
		catalog:   catalog.NewService(pool),
		inventory: inv,
		enquiries: enquiry.NewService(pool, provider, inv, now),
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
		r.Post("/bookings", h(a.createBooking))
		r.Get("/bookings/{ref}", h(a.getBooking))
		r.With(a.requirePermission(auth.PermOwnBookingsRead)).Get("/me/bookings", h(a.myBookings))

		// Payments: the mock gateway page, its simulator, and the signed webhook.
		r.Get("/payments/{orderId}", h(a.paymentPage))
		r.Post("/payments/{orderId}/simulate", h(a.simulatePayment))
		r.Post("/payments/webhook", h(a.paymentWebhook))

		// Enquiries from the website's forms.
		r.Post("/enquiries", h(a.createEnquiry))

		// Staff console. Each route names the permission it needs.
		r.Route("/staff", func(r chi.Router) {
			r.With(a.requirePermission(auth.PermOutboxRead)).Get("/outbox", h(a.listOutbox))
			r.With(a.requirePermission(auth.PermBookingsRead)).Get("/bookings", h(a.staffBookings))
			r.With(a.requirePermission(auth.PermBookingsExport)).Get("/bookings/export", h(a.exportBookings))
			r.With(a.requirePermission(auth.PermRatesRead)).Get("/rates", h(a.ratesGrid))
			r.With(a.requirePermission(auth.PermRatesRead)).Put("/rates", h(a.updateRates)) // the handler checks write permissions per field
			r.With(a.requirePermission(auth.PermDashboardRead)).Get("/dashboard", h(a.dashboard))

			r.Group(func(r chi.Router) {
				r.Use(a.requirePermission(auth.PermEnquiriesRead))
				r.Post("/inbound", h(a.simulateInbound))
				r.Get("/enquiries", h(a.listEnquiries))
				r.Get("/enquiries/{ref}", h(a.getEnquiry))
				r.Patch("/enquiries/{ref}/fields", h(a.correctEnquiry))
				r.Put("/enquiries/{ref}/draft", h(a.editDraft))
				r.Post("/enquiries/{ref}/status", h(a.setEnquiryStatus))
				r.Post("/enquiries/{ref}/retry-ai", h(a.retryAI))
				r.With(a.requirePermission(auth.PermEnquiryReroute)).Post("/enquiries/{ref}/route", h(a.routeEnquiry))
				r.With(a.requirePermission(auth.PermEnquirySend)).Post("/enquiries/{ref}/send", h(a.sendReply))
			})
		})
	})
	return r
}

// SweepBookings expires unpaid bookings in the background. Holds stop counting the moment
// they expire regardless; this only tidies statuses for staff screens.
func (a *API) SweepBookings(ctx context.Context, every, grace time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := a.bookings.Sweep(ctx, grace); err != nil {
				a.log.Error("booking sweep failed", "err", err)
			} else if n > 0 {
				a.log.Info("expired unpaid bookings", "count", n)
			}
		}
	}
}
