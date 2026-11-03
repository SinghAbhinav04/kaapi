// Command api serves the Kaapi Estate HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kaapi/api/internal/app"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/logging"
	"kaapi/api/internal/seed"
	"kaapi/api/internal/transport/httpapi"
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
)

func main() {
	if err := config.LoadDotEnv(config.DotEnvFile); err != nil { // local development only
		log.Fatal(err)
	}
	cfg, err := config.Load()
	log := logging.New(cfg.LogLevel)
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

	a := app.New(cfg, log, pool, time.Now)
	// Personal data written before encryption existed is encrypted now (a no-op once done).
	if n, err := seed.EncryptExisting(ctx, pool); err != nil {
		log.Error("encrypting existing personal data failed", "err", err)
		os.Exit(1)
	} else if n > 0 {
		log.Info("encrypted existing personal data", "rows", n)
	}
	if cfg.WorkerInProcess { // local development: one process does everything
		go a.RunWorkers(ctx)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(a).Router(),
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
