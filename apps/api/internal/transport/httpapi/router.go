// Package httpapi is the HTTP transport: routes, request/response shapes (DTOs) and the
// gateway middleware. Domain packages stay free of net/http; handlers translate between
// JSON and domain calls and return errors for httpx.WriteError to render. Services come
// ready-built from internal/app.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/access"
	"kaapi/api/internal/admin"
	"kaapi/api/internal/analytics"
	"kaapi/api/internal/app"
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
	access    *access.Service
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
	hub       *hub
}

// New exposes the app's services over HTTP.
func New(a *app.App) *API {
	return &API{cfg: a.Config, log: a.Log, pool: a.Pool, access: a.Access, admin: a.Admin, analytics: a.Analytics, auth: a.Auth,
		bookings: a.Bookings, catalog: a.Catalog, enquiries: a.Enquiries, fx: a.FX, gmail: a.Gmail, inventory: a.Inventory,
		outbox: a.Outbox, retention: a.Retention, hub: newHub(a.Pool, a.Log)}
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
	r.Use(timeoutExceptStreams(a.cfg.RequestTimeout))
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
			r.With(a.requirePermission(auth.PermEnquiriesRead)).Post("/integrations/gmail/sync", h(a.gmailSync))
			r.Group(func(r chi.Router) {
				r.Use(a.requirePermission(auth.PermIntegrations))
				r.Post("/integrations/gmail/connect", h(a.gmailConnect))
				r.Delete("/integrations/gmail", h(a.gmailDisconnect))
			})
			r.With(a.requirePermission(auth.PermEnquirySend)).Post("/messages", h(a.composeMessage))

			// Team and access: the administrator's console for people, roles and permissions.
			r.Group(func(r chi.Router) {
				r.Use(a.requirePermission(auth.PermUsersManage))
				r.Get("/access", h(a.accessOverview))
				r.Post("/roles", h(a.createRole))
				r.Put("/roles/{id}", h(a.updateRole))
				r.Delete("/roles/{id}", h(a.deleteRole))
				r.Get("/users", h(a.listMembers))
				r.Post("/users", h(a.createMember))
				r.Patch("/users/{id}", h(a.updateMember))
			})

			r.Group(func(r chi.Router) {
				r.Use(a.requirePermission(auth.PermEnquiriesRead))
				r.Post("/inbound", h(a.simulateInbound))
				r.Get("/enquiries", h(a.listEnquiries))
				r.Get("/inbox/stream", h(a.inboxStream))
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
