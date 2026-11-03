// Command api serves the Kaapi Estate HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kaapi/api/internal/api"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/db"
)

// Server timeouts: generous enough for slow hill-station networks, short enough that
// stalled connections can't pile up.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownGrace     = 20 * time.Second
	startupTimeout    = 60 * time.Second

	// Unpaid bookings are expired this long after their hold ends; late payments inside
	// the grace period still go through if the room is free.
	sweepEvery = time.Minute
	sweepGrace = 30 * time.Minute

	// The enquiry worker wakes on every new message; this tick is only a safety net.
	enquiryTick = 15 * time.Second

	// Retention windows are months long, so a few runs a day is plenty.
	retentionEvery = 6 * time.Hour

	// The ECB publishes once a working day; a few fetches a day catch it without hammering.
	fxEvery = 6 * time.Hour
)

func main() {
	if err := config.LoadDotEnv(config.DotEnvFile); err != nil { // local development only
		log.Fatal(err)
	}
	cfg, err := config.Load()
	log := newLogger(cfg.LogLevel)
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	if cfg.MigrateOnStart {
		if err := db.Migrate(startCtx, cfg.DatabaseURL); err != nil {
			log.Error("migrations failed", "err", err)
			os.Exit(1)
		}
	}
	pool, err := db.Connect(startCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Error("database unavailable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	app := api.New(cfg, log, pool, time.Now)
	go app.SweepBookings(ctx, sweepEvery, sweepGrace)
	go app.RunEnquiryWorker(ctx, enquiryTick)
	go app.SweepRetention(ctx, retentionEvery)
	go app.RunGmail(ctx)
	go app.RefreshFX(ctx, fxEvery)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	go func() {
		log.Info("listening", "addr", srv.Addr, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
