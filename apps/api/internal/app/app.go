// Package app is the composition root. It builds every service from config exactly once,
// and the HTTP API (cmd/api), the background worker (cmd/worker) and the tests all start
// from it, so they can never be wired differently.
package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/admin"
	"kaapi/api/internal/ai"
	"kaapi/api/internal/analytics"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/booking"
	"kaapi/api/internal/catalog"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/gmail"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/retention"
)

// aiDiscoveryTimeout bounds the background call that lists the models each key can use.
const aiDiscoveryTimeout = time.Minute

// App is every service, built and wired.
type App struct {
	Config config.Config
	Log    *slog.Logger
	Pool   *pgxpool.Pool

	Admin     *admin.Service
	Analytics *analytics.Service
	Auth      *auth.Service
	Bookings  *booking.Service
	Catalog   *catalog.Service
	Enquiries *enquiry.Service
	Gmail     *gmail.Service
	Inventory *inventory.Service
	Outbox    *outbox.Service
	Retention *retention.Service
}

func New(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, now func() time.Time) *App {
	inv := inventory.NewService(pool, now)
	a := &App{
		Config:    cfg,
		Log:       log,
		Pool:      pool,
		Admin:     admin.NewService(pool),
		Analytics: analytics.NewService(pool, now),
		Auth:      auth.NewService(pool, cfg.JWESecret, now),
		Bookings:  booking.NewService(pool, now),
		Catalog:   catalog.NewService(pool),
		Inventory: inv,
		Enquiries: enquiry.NewService(pool, startAI(cfg.AI, log), inv, now),
		Outbox:    outbox.NewService(pool),
		Retention: retention.NewService(pool, now),
		Gmail:     gmail.NewService(pool, cfg.Gmail, cfg.JWESecret, now, log),
	}
	// One mailbox serves both directions: imported mail becomes enquiries, and replies and
	// staff-written messages go out through it. Unconnected, everything stays simulated.
	a.Gmail.SetInbox(mailInbox{a})
	a.Enquiries.SetMailer(a.Gmail)
	a.Outbox.SetMailer(a.Gmail)
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
