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

	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/pii"
	"kaapi/api/internal/seed"
)

func main() {
	if err := config.LoadDotEnv(config.DotEnvFile); err != nil { // local development only
		log.Fatal(err)
	}
	dir := flag.String("data", "seed/data", "fixture folder")
	reset := flag.Bool("reset", false, "wipe existing data before seeding")
	adminOnly := flag.Bool("admin", false, "only create or update the administrator from SEED_ADMIN_* (keeps all other data)")
	flag.Parse()
	admin := adminFromEnv()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := db.Migrate(ctx, url); err != nil {
		log.Fatal(err)
	}
	// Personal data is encrypted with the API's key: PII_KEY, else (development) one derived
	// from JWE_SECRET, exactly as the API does. Without either, nothing seeded would decrypt.
	if os.Getenv(config.EnvPIIKey) == "" && len(os.Getenv(config.EnvJWESecret)) < 32 {
		log.Fatalf("set %s (or %s for development) so seeded personal data uses the API's key", config.EnvPIIKey, config.EnvJWESecret)
	}
	if err := pii.Configure(config.PIIKey(os.Getenv(config.EnvPIIKey), []byte(os.Getenv(config.EnvJWESecret)))); err != nil {
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

	// A database seeded before encryption existed: encrypt what's there first.
	if n, err := seed.EncryptExisting(ctx, pool); err != nil {
		log.Fatal(err)
	} else if n > 0 {
		fmt.Printf("encrypted %d rows of existing personal data\n", n)
	}

	if *adminOnly {
		if admin == nil {
			log.Fatalf("set %s and %s to create the administrator", envAdminEmail, envAdminPassword)
		}
		if err := seed.EnsureAdmin(ctx, pool, *admin); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("administrator %s is ready\n", admin.Email)
		return
	}

	sum, err := seed.Run(ctx, pool, fx, seed.Options{DemoPassword: os.Getenv("SEED_DEMO_PASSWORD"), Reset: *reset, Admin: admin})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("seeded: %d room types, %d inventory nights, %d nightly rates, %d bookings (%d confirmed), %d enquiries, %d users\n",
		sum.RoomTypes, sum.Nights, sum.Rates, sum.Bookings, sum.Confirmed, sum.Enquiries, sum.Users)
}

// The deployment's own administrator, kept out of the fixtures (real email, real password).
const (
	envAdminEmail    = "SEED_ADMIN_EMAIL"
	envAdminPassword = "SEED_ADMIN_PASSWORD"
	envAdminName     = "SEED_ADMIN_NAME"
)

func adminFromEnv() *seed.Account {
	email, password := os.Getenv(envAdminEmail), os.Getenv(envAdminPassword)
	if email == "" || password == "" {
		return nil
	}
	return &seed.Account{Email: email, Password: password, Name: os.Getenv(envAdminName)}
}
