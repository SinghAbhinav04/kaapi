// Package app is the composition root. It builds every service from config exactly once,
// and the HTTP API (cmd/api), the background worker (cmd/worker) and the tests all start
// from it, so they can never be wired differently.
package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/access"
	"kaapi/api/internal/admin"
	"kaapi/api/internal/ai"
	"kaapi/api/internal/analytics"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/campaign"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/fx"
	"kaapi/api/internal/gmail"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/platform/cache"
	"kaapi/api/internal/platform/changes"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/emailtpl"
	"kaapi/api/internal/platform/pii"
	"kaapi/api/internal/retention"
)

// estateDirections opens the estate in Google Maps (the site's map uses the same pin).
const estateDirections = "https://www.google.com/maps/search/?api=1&query=Baba+Budangiri%2C+Chikkamagaluru%2C+Karnataka"

// aiDiscoveryTimeout bounds the background call that lists the models each key can use.
const aiDiscoveryTimeout = time.Minute

// App is every service, built and wired.
type App struct {
	Config config.Config
	Log    *slog.Logger
	Pool   *pgxpool.Pool

	Access    *access.Service
	Admin     *admin.Service
	Analytics *analytics.Service
	// Changes is the change feed (triggers → pg_notify): it invalidates caches and tells open
	// staff consoles what to refetch. cmd/api runs it.
	Changes   *changes.Feed
	Auth      *auth.Service
	Bookings  *booking.Service
	Campaigns *campaign.Service
	Catalog   *catalog.Service
	Enquiries *enquiry.Service
	FX        *fx.Service
	Gmail     *gmail.Service
	Inventory *inventory.Service
	Outbox    *outbox.Service
	Retention *retention.Service
}

// maxCachedReports bounds the analytics cache: one entry per date range staff look at.
const maxCachedReports = 64

func New(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, now func() time.Time) *App {
	key := cfg.PIIKey
	if len(key) == 0 { // a hand-built config (tests): the same derivation config.Load uses
		key = config.PIIKey("", cfg.JWESecret)
	}
	if err := pii.Configure(key); err != nil {
		panic(err) // config.Load already refused a short key; only a hand-built config gets here
	}
	inv := inventory.NewService(pool, now)
	a := &App{
		Config:    cfg,
		Log:       log,
		Pool:      pool,
		Access:    access.NewService(pool, now),
		Admin:     admin.NewService(pool),
		Analytics: analytics.NewService(pool, now),
		Auth:      auth.NewService(pool, cfg.JWESecret, now),
		Bookings:  booking.NewService(pool, now),
		Catalog:   catalog.NewService(pool),
		Inventory: inv,
		Enquiries: enquiry.NewService(pool, startAI(cfg.AI, log), inv, now),
		FX:        fx.NewService(pool, cfg.FXBaseURL),
		Outbox:    outbox.NewService(pool),
		Retention: retention.NewService(pool, now),
		Gmail:     gmail.NewService(pool, cfg.Gmail, cfg.JWESecret, now, log),
	}
	// One mailbox serves both directions: imported mail becomes enquiries, and replies and
	// staff-written messages go out through it. Unconnected, everything stays simulated.
	a.Gmail.SetInbox(mailInbox{a})
	a.Enquiries.SetMailer(a.Gmail)
	a.Outbox.SetMailer(a.Gmail)
	// Offers staff choose to announce reach opted-in guests: the campaign service watches
	// published offers, and a booking's opt-in is recorded through it.
	a.Campaigns = campaign.NewService(pool, now, cfg.AppOrigin)
	a.Admin.Watch(offerWatcher{a.Campaigns})
	a.Bookings.SetConsents(a.Campaigns)
	// Emails' buttons point at the site; directions at the estate (an assumed pin, as on the site).
	links := emailtpl.Links{Site: cfg.AppOrigin, Directions: estateDirections}
	a.Outbox.SetLinks(links)
	a.Enquiries.SetLinks(links)
	a.Changes = changes.New(pool, log)
	if cfg.AnalyticsCacheTTL > 0 {
		a.Analytics.SetCache(cache.New[string, analytics.Report](cfg.AnalyticsCacheTTL, maxCachedReports))
		a.Changes.OnChange(func(topic string) {
			switch topic {
			case changes.Bookings, changes.Inventory, changes.Enquiries, changes.All:
				a.Analytics.Invalidate()
			}
		})
	}
	return a
}

// startAI builds the model chain, or nil (keyword rules and template drafts only).
func startAI(c config.AI, log *slog.Logger) ai.Provider {
	opts := AIOptions(c)
	chain := ai.Start(opts)
	if chain == nil {
		return nil
	}
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
	return chain
}

// AIOptions maps config onto the chain's options.
func AIOptions(c config.AI) ai.Options {
	return ai.Options{GeminiKey: c.GeminiKey, GeminiBaseURL: c.GeminiBaseURL, GeminiModels: c.GeminiModels, GeminiRPM: c.GeminiRPM,
		GroqKey: c.GroqKey, GroqBaseURL: c.GroqBaseURL, GroqModels: c.GroqModels, GroqRPM: c.GroqRPM}
}

// offerWatcher adapts the campaign service to the offers service's observer interface.
type offerWatcher struct{ c *campaign.Service }

func (w offerWatcher) OfferPublished(ctx context.Context, tx pgx.Tx, o admin.Offer, by uuid.UUID) error {
	return w.c.OfferPublished(ctx, tx, campaign.Offer{ID: o.ID, Code: o.Code, Description: o.Description,
		StayFrom: o.StayFrom, StayTo: o.StayTo, BookTo: o.BookTo, MinNights: o.MinNights}, by)
}
