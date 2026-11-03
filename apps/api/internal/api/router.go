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
	"kaapi/api/internal/analytics"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/fx"
	"kaapi/api/internal/gmail"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/httpx"
	"kaapi/api/internal/retention"
)

// API holds every dependency the handlers need.
type API struct {
	cfg       config.Config
	log       *slog.Logger
	pool      *pgxpool.Pool
	admin     *admin.Service
	analytics *analytics.Service
	auth      *auth.Service
	bookings  *booking.Service
	catalog   *catalog.Service
	enquiries *enquiry.Service
	fx        *fx.Service
	gmail     *gmail.Service
	inventory *inventory.Service
	outbox    *outbox.Service
	retention *retention.Service
}

func New(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, now func() time.Time) *API {
	inv := inventory.NewService(pool, now)
	var provider ai.Provider // nil: keyword rules and template drafts only
	opts := AIOptions(cfg.AI)
	if chain := ai.Start(opts); chain != nil {
		provider = chain
		log.Info("ai chain ready", "models", chain.Models())
		if ai.NeedsDiscovery(opts) { // fill in every model the keys allow, without delaying start-up
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), aiDiscoveryTimeout)
				defer cancel()
				for _, w := range chain.Refresh(ctx, opts) {
					log.Warn("ai", "warning", w)
				}
				log.Info("ai models discovered", "models", chain.Models())
			}()
		}
	}
	a := &API{
		cfg:       cfg,
		log:       log,
		pool:      pool,
		admin:     admin.NewService(pool),
		analytics: analytics.NewService(pool, now),
		auth:      auth.NewService(pool, cfg.JWESecret, now),
		bookings:  booking.NewService(pool, now),
		catalog:   catalog.NewService(pool),
		inventory: inv,
		enquiries: enquiry.NewService(pool, provider, inv, now),
		fx:        fx.NewService(pool, cfg.FXBaseURL),
		outbox:    outbox.NewService(pool),
		retention: retention.NewService(pool, now),
		gmail:     gmail.NewService(pool, cfg.Gmail, cfg.JWESecret, now, log),
	}
	// One mailbox serves both directions: imported mail becomes enquiries, and replies and
	// staff-written messages go out through it. Unconnected, everything stays simulated.
	a.gmail.SetDeliver(a.deliverEmail)
	a.enquiries.SetMailer(a.gmail)
	a.outbox.SetMailer(a.gmail)
	return a
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
		r.Get("/fx-rates", h(a.fxRates))
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

		// Google sends the admin's browser back here after the Gmail consent screen.
		r.Get("/integrations/gmail/callback", h(a.gmailCallback))

		// Staff console. Each route names the permission it needs.
		r.Route("/staff", func(r chi.Router) {
			r.With(a.requirePermission(auth.PermOutboxRead)).Get("/outbox", h(a.listOutbox))
			r.With(a.requirePermission(auth.PermBookingsRead)).Get("/bookings", h(a.staffBookings))
			r.With(a.requirePermission(auth.PermBookingsExport)).Get("/bookings/export", h(a.exportBookings))
			r.With(a.requirePermission(auth.PermRatesRead)).Get("/rates", h(a.ratesGrid))
			r.With(a.requirePermission(auth.PermRatesRead)).Put("/rates", h(a.updateRates)) // the handler checks write permissions per field
			r.With(a.requirePermission(auth.PermDashboardRead)).Get("/dashboard", h(a.dashboard))
			r.With(a.requirePermission(auth.PermRetentionRun)).Post("/retention/run", h(a.runRetention))
			r.With(a.requirePermission(auth.PermAnalyticsRead)).Get("/analytics", h(a.analyticsReport))
			r.With(a.requirePermission(auth.PermEnquiriesRead)).Get("/integrations/gmail", h(a.gmailStatus))
			r.Group(func(r chi.Router) {
				r.Use(a.requirePermission(auth.PermIntegrations))
				r.Post("/integrations/gmail/connect", h(a.gmailConnect))
				r.Post("/integrations/gmail/sync", h(a.gmailSync))
				r.Delete("/integrations/gmail", h(a.gmailDisconnect))
			})
			r.With(a.requirePermission(auth.PermEnquirySend)).Post("/messages", h(a.composeMessage))

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

// SweepRetention archives finished bookings and deletes expired personal data: once at
// start-up, then on every tick. Each run is idempotent, so a missed or doubled tick is harmless.
func (a *API) SweepRetention(ctx context.Context, every time.Duration) {
	run := func() {
		if r, err := a.retention.Sweep(ctx); err != nil {
			a.log.Error("retention sweep failed", "err", err)
		} else if r.Total() > 0 {
			a.log.Info("retention sweep", "result", r)
		}
	}
	run()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

// RefreshFX keeps the exchange rates current: once at start-up, then on every tick. A failed
// fetch only logs; the site shows INR alone once the stored rates go stale.
func (a *API) RefreshFX(ctx context.Context, every time.Duration) {
	run := func() {
		if n, err := a.fx.Refresh(ctx); err != nil {
			a.log.Warn("fx refresh failed", "err", err)
		} else {
			a.log.Info("fx rates refreshed", "currencies", n)
		}
	}
	run()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

// FX exposes the rate service to tests, which refresh it from a fake.
func (a *API) FX() *fx.Service { return a.fx }

// aiDiscoveryTimeout bounds the background call that lists the models each key can use.
const aiDiscoveryTimeout = time.Minute

// AIOptions maps config onto the chain's options.
func AIOptions(c config.AI) ai.Options {
	return ai.Options{GeminiKey: c.GeminiKey, GeminiBaseURL: c.GeminiBaseURL, GeminiModels: c.GeminiModels, GeminiRPM: c.GeminiRPM,
		GroqKey: c.GroqKey, GroqBaseURL: c.GroqBaseURL, GroqModels: c.GroqModels, GroqRPM: c.GroqRPM}
}
