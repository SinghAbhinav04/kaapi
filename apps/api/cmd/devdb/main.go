// Command devdb runs a real Postgres 17 for local development and tests, with nothing to install.
// The first run downloads the Postgres binaries (about 30 MB) into the user cache.
//
//	go run ./cmd/devdb            # keeps running until Ctrl+C
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
)

func main() {
	port := flag.Uint("port", 54329, "port to listen on")
	dir := flag.String("data", ".devdata", "folder for the database files")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatal(err)
	}
	cfg := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V17).
		Port(uint32(*port)).
		Database("kaapi").
		Username("kaapi").
		Password("kaapi").
		Locale("C"). // Windows would otherwise pick WIN1252, which can't store ₹ or Hindi enquiries
		Encoding("UTF8").
		DataPath(filepath.Join(root, "pg")).
		RuntimePath(filepath.Join(root, "runtime")).
		Logger(os.Stderr)

	pg := embeddedpostgres.NewDatabase(cfg)
	url := fmt.Sprintf("postgres://kaapi:kaapi@localhost:%d/kaapi?sslmode=disable", *port)
	if err := pg.Start(); err != nil {
		// Already running (e.g. from an earlier session)? Then just make sure the test database exists.
		if ensureDatabase(url, testDatabase) == nil {
			fmt.Printf("Postgres is already running on port %d; the %s database is ready.\n", *port, testDatabase)
			return
		}
		log.Fatalf("start postgres: %v", err)
	}
	// Tests get their own database: they wipe and reseed it, so they must never share the
	// one a running dev server is using.
	if err := ensureDatabase(url, testDatabase); err != nil {
		log.Printf("could not create the test database: %v", err)
	}
	fmt.Printf("\nPostgres 17 is running.\nDATABASE_URL=%s\nTEST_DATABASE_URL=postgres://kaapi:kaapi@localhost:%d/%s?sslmode=disable\nPress Ctrl+C to stop.\n",
		url, *port, testDatabase)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	if err := pg.Stop(); err != nil {
		log.Fatalf("stop postgres: %v", err)
	}
}

const testDatabase = "kaapi_test"

func ensureDatabase(url, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil || exists {
		return err
	}
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C'")
	return err
}
