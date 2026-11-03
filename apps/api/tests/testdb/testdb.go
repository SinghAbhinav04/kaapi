// Package testdb gives integration tests a freshly migrated and seeded database.
//
// Start one with `go run ./cmd/devdb`, then run tests with
// TEST_DATABASE_URL=postgres://kaapi:kaapi@localhost:54329/kaapi_test?sslmode=disable
// (devdb creates kaapi_test so tests never wipe the database a dev server is using).
// Tests are skipped when the variable is not set.
package testdb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/pii"
	"kaapi/api/internal/seed"
)

// Seeded migrates, wipes and reseeds the database, and returns a pool sized for
// concurrency tests. Tests using it must not run in parallel with each other.
// JWESecret is the token secret every test API uses (apitest.TestConfig). The PII key is
// derived from it exactly as the API derives it, so seeded rows decrypt in the API.
const JWESecret = "test-only-jwe-secret-0123456789abcdef"

func Seeded(t *testing.T) (context.Context, *pgxpool.Pool) {
	if err := pii.Configure(config.PIIKey("", []byte(JWESecret))); err != nil {
		t.Fatal(err)
	}
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; start a database with `go run ./cmd/devdb`")
	}
	// Tests truncate every table. Refuse anything that looks like a hosted database.
	if strings.Contains(url, "supabase.co") || strings.Contains(url, "pooler.supabase.com") {
		t.Fatal("TEST_DATABASE_URL points at Supabase; tests wipe their database, so use a throwaway one (go run ./cmd/devdb)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	if err := db.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 30
	cfg.ConnConfig.Tracer = db.Tracer() // as in production, so Server-Timing counts queries
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	fx, err := seed.Load(fixtureDir())
	if err != nil {
		t.Fatal(err)
	}
	// A re-seed keeps the connected mailbox on purpose (it should survive resets); tests
	// still start with none.
	if _, err := pool.Exec(ctx, `TRUNCATE integrations`); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Run(ctx, pool, fx, seed.Options{DemoPassword: "test-only-password", Reset: true}); err != nil {
		t.Fatal(err)
	}
	return ctx, pool
}

func fixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "seed", "data")
}
