// Command gmail-auth connects the resort's mailbox once. It prints (and opens) Google's
// consent link; after "Allow", the running app stores the grant, and this command saves it
// to .env as GMAIL_REFRESH_TOKEN so the API is connected on every start from then on.
//
//	go run ./cmd/gmail-auth         # with the API and web app running
//	go run ./cmd/gmail-auth -save   # already connected (e.g. from the console): just save it to .env
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/gmail"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/db"
)

const (
	waitFor   = 5 * time.Minute
	pollEvery = 2 * time.Second
	envKey    = config.EnvGmailRefreshToken
)

func main() {
	saveOnly := flag.Bool("save", false, "save the current connection to .env without asking Google again")
	flag.Parse()
	if err := config.LoadDotEnv(config.DotEnvFile); err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.Gmail.Configured() {
		log.Fatalf("set %s and %s first", config.EnvGoogleClientID, config.EnvGoogleClientSecret)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitFor+time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if *saveOnly {
		svc := gmail.NewService(pool, cfg.Gmail, cfg.JWESecret, time.Now, slog.New(slog.DiscardHandler))
		token, email, err := svc.RefreshToken(ctx)
		if err != nil {
			log.Fatalf("nothing to save: %v", err)
		}
		if err := saveEnv(config.DotEnvFile, envKey, token); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Saved the connection for %s to %s as %s.\n", email, config.DotEnvFile, envKey)
		return
	}

	// The consent is recorded against the administrator.
	var admin uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE role = 'ADMIN' AND is_active
		ORDER BY lower(email) = lower($1) DESC LIMIT 1`, os.Getenv("SEED_ADMIN_EMAIL")).Scan(&admin); err != nil {
		log.Fatalf("no administrator account: run go run ./cmd/seed -admin first (%v)", err)
	}

	svc := gmail.NewService(pool, cfg.Gmail, cfg.JWESecret, time.Now, slog.New(slog.DiscardHandler))
	before, _, _ := svc.RefreshToken(ctx)
	link, err := svc.ConsentURL(admin)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Open this link, choose the resort's Gmail account and click Allow:\n\n  %s\n\nWaiting up to %s…\n", link, waitFor)
	openBrowser(link)

	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {
		token, email, err := svc.RefreshToken(ctx)
		if err != nil && !errors.Is(err, gmail.ErrNotConnected) {
			log.Fatal(err)
		}
		if token != "" && token != before {
			if err := saveEnv(config.DotEnvFile, envKey, token); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Connected as %s. Saved %s to %s; the API stays connected on every start.\n", email, envKey, config.DotEnvFile)
			return
		}
		time.Sleep(pollEvery)
	}
	log.Fatal("no answer from Google in time; is the web app running on APP_ORIGIN? Run this again.")
}

// saveEnv sets key in the .env file, replacing an existing line or adding one.
func saveEnv(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	found := false
	for i, l := range lines {
		if strings.HasPrefix(l, key+"=") {
			lines[i], found = key+"="+value, true
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start() // the printed link is the fallback
}
