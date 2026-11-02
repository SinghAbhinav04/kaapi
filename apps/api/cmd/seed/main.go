// Command seed applies migrations, then loads the demo data from seed/data.
//
//	DATABASE_URL=... SEED_DEMO_PASSWORD=... go run ./cmd/seed [-reset]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/db"
	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/seed"
)

func main() {
	dir := flag.String("data", "seed/data", "fixture folder")
	reset := flag.Bool("reset", false, "wipe existing data before seeding")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := db.Migrate(ctx, url); err != nil {
		log.Fatal(err)
	}
	fx, err := seed.Load(*dir)
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.Connect(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	sum, err := seed.Run(ctx, pool, fx, seed.Options{DemoPassword: os.Getenv("SEED_DEMO_PASSWORD"), Reset: *reset})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("seeded: %d room types, %d inventory nights, %d nightly rates, %d bookings (%d confirmed), %d enquiries, %d users\n",
		sum.RoomTypes, sum.Nights, sum.Rates, sum.Bookings, sum.Confirmed, sum.Enquiries, sum.Users)
}
