// Package config reads the API's settings from environment variables, once, at start-up.
// Missing required values stop the process with a message naming every one of them.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env names. deployment.md lists these; values never live in the repository.
const (
	EnvPort              = "PORT"
	EnvDatabaseURL       = "DATABASE_URL"
	EnvAppOrigin         = "APP_ORIGIN"
	EnvJWESecret         = "JWE_SECRET"
	EnvGeminiAPIKey      = "GEMINI_API_KEY"
	EnvGeminiModel       = "GEMINI_MODEL"
	EnvPaymentSecret     = "MOCK_PAYMENT_WEBHOOK_SECRET"
	EnvMigrateOnStart    = "MIGRATE_ON_START"
	EnvEnvironment       = "APP_ENV"
	EnvCookieSecure      = "COOKIE_SECURE"
	EnvLogLevel          = "LOG_LEVEL"
	EnvNativeAppOrigins  = "NATIVE_APP_ORIGINS"
	EnvRequestTimeoutSec = "REQUEST_TIMEOUT_SECONDS"
)

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

// minSecretBytes is the shortest secret accepted for token encryption and webhook signing.
const minSecretBytes = 32

type Config struct {
	Port             string
	DatabaseURL      string
	AppOrigin        string   // the web app; cookies and CSRF origin checks are tied to it
	NativeAppOrigins []string // Capacitor shells allowed by CORS
	JWESecret        []byte
	PaymentSecret    []byte
	GeminiAPIKey     string // empty: the keyword classifier is used on its own
	GeminiModel      string
	MigrateOnStart   bool
	Environment      Environment
	CookieSecure     bool
	LogLevel         string
	RequestTimeout   time.Duration
}

func (c Config) IsProduction() bool { return c.Environment == EnvProduction }

// Load reads and validates the environment.
func Load() (Config, error) {
	c := Config{
		Port:           get(EnvPort, "8080"),
		DatabaseURL:    os.Getenv(EnvDatabaseURL),
		AppOrigin:      strings.TrimRight(get(EnvAppOrigin, "http://localhost:3000"), "/"),
		JWESecret:      []byte(os.Getenv(EnvJWESecret)),
		PaymentSecret:  []byte(os.Getenv(EnvPaymentSecret)),
		GeminiAPIKey:   os.Getenv(EnvGeminiAPIKey),
		GeminiModel:    get(EnvGeminiModel, "gemini-2.5-flash"),
		MigrateOnStart: get(EnvMigrateOnStart, "true") == "true",
		Environment:    Environment(get(EnvEnvironment, string(EnvDevelopment))),
		LogLevel:       get(EnvLogLevel, "info"),
	}
	c.CookieSecure = get(EnvCookieSecure, strconv.FormatBool(c.IsProduction())) == "true"
	for _, o := range strings.Split(os.Getenv(EnvNativeAppOrigins), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.NativeAppOrigins = append(c.NativeAppOrigins, o)
		}
	}
	secs, err := strconv.Atoi(get(EnvRequestTimeoutSec, "15"))
	if err != nil || secs < 1 {
		return c, fmt.Errorf("%s must be a positive number of seconds", EnvRequestTimeoutSec)
	}
	c.RequestTimeout = time.Duration(secs) * time.Second

	var problems []string
	if c.DatabaseURL == "" {
		problems = append(problems, EnvDatabaseURL+" is required")
	}
	if len(c.JWESecret) < minSecretBytes {
		problems = append(problems, fmt.Sprintf("%s must be at least %d bytes", EnvJWESecret, minSecretBytes))
	}
	if len(c.PaymentSecret) < minSecretBytes {
		problems = append(problems, fmt.Sprintf("%s must be at least %d bytes", EnvPaymentSecret, minSecretBytes))
	}
	if c.Environment != EnvDevelopment && c.Environment != EnvProduction {
		problems = append(problems, EnvEnvironment+" must be development or production")
	}
	if len(problems) > 0 {
		return c, errors.New("config: " + strings.Join(problems, "; "))
	}
	return c, nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
