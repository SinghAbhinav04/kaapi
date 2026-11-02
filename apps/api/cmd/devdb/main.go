// Command devdb runs a real Postgres 17 for local development and tests, with nothing to install.
// The first run downloads the Postgres binaries (about 30 MB) into the user cache.
//
//	go run ./cmd/devdb            # keeps running until Ctrl+C
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
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
	if err := pg.Start(); err != nil {
		log.Fatalf("start postgres: %v", err)
	}
	fmt.Printf("\nPostgres 17 is running.\nDATABASE_URL=postgres://kaapi:kaapi@localhost:%d/kaapi?sslmode=disable\nPress Ctrl+C to stop.\n", *port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	if err := pg.Stop(); err != nil {
		log.Fatalf("stop postgres: %v", err)
	}
}
