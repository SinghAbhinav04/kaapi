// Command worker runs the background jobs: the AI enquiry queue, the Gmail poller, the
// unpaid-booking sweep and the retention sweep. In production it runs as its own service
// (one always-on instance) so the API can scale to zero; it answers /healthz on PORT so a
// platform like Cloud Run can keep it alive and restart it if it stops.
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
)

const (
	startupTimeout = 60 * time.Second
	shutdownGrace  = 10 * time.Second
	readTimeout    = 5 * time.Second
)

func main() {
	if err := config.LoadDotEnv(config.DotEnvFile); err != nil { // local development only
		log.Fatal(err)
	}
	cfg, err := config.Load()
	logger := logging.New(cfg.LogLevel)
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The API owns migrations; the worker only waits for the database.
	startCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	pool, err := db.Connect(startCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		logger.Error("database unavailable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	health := &http.Server{Addr: ":" + cfg.Port, ReadHeaderTimeout: readTimeout, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})}
	go func() {
		if err := health.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server stopped", "err", err)
			stop()
		}
	}()

	logger.Info("worker running", "env", cfg.Environment)
	app.New(cfg, logger, pool, time.Now).RunWorkers(ctx)

	logger.Info("worker shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	_ = health.Shutdown(shutdownCtx)
}
