// Package config reads the API's settings from environment variables, once, at start-up.
// Missing required values stop the process with a message naming every one of them.
package config

import (
	"crypto/hmac"
	"crypto/sha256"
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
	EnvGeminiModels      = "GEMINI_MODELS" // optional, comma-separated; empty = every text model the key can use
	EnvGeminiBaseURL     = "GEMINI_BASE_URL"
	EnvGeminiRPM         = "GEMINI_RPM" // per model
	EnvGroqAPIKey        = "GROQ_API_KEY"
	EnvGroqModels        = "GROQ_MODELS"
	EnvGroqBaseURL       = "GROQ_BASE_URL"
	EnvGroqRPM           = "GROQ_RPM"
	EnvPaymentSecret     = "MOCK_PAYMENT_WEBHOOK_SECRET"
	EnvMigrateOnStart    = "MIGRATE_ON_START"
	EnvEnvironment       = "APP_ENV"
	EnvCookieSecure      = "COOKIE_SECURE"
	EnvLogLevel          = "LOG_LEVEL"
	EnvNativeAppOrigins  = "NATIVE_APP_ORIGINS"
	EnvRequestTimeoutSec = "REQUEST_TIMEOUT_SECONDS"
	EnvBrowserAPIPrefix  = "BROWSER_API_PREFIX"      // where browsers reach the API; default /api (the web app's proxy)
	EnvFXBaseURL         = "FX_API_BASE_URL"         // exchange rates; empty = Frankfurter (ECB). Tests point it at a fake
	EnvWorkerInProcess   = "WORKER_IN_PROCESS"       // run background jobs inside cmd/api (default: on in development)
	EnvRateLimits        = "RATE_LIMITS"             // "off" disables the gateway's limits (load tests); on by default
	EnvTrustedProxyHops  = "TRUSTED_PROXY_HOPS"      // proxies in front of the API that append X-Forwarded-For; default 1 (the web proxy)
	EnvPIIKey            = "PII_KEY"                 // encrypts guests' personal data at rest; 32+ bytes, required in production
	EnvAnalyticsCacheSec = "ANALYTICS_CACHE_SECONDS" // how long a computed analytics report is kept (the change feed drops it sooner); 0 = off

	// Gmail: the inbox reads and replies from a real mailbox once an admin connects it.
	EnvGoogleClientID     = "GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret = "GOOGLE_CLIENT_SECRET"
	EnvGmailRedirectURL   = "GMAIL_REDIRECT_URL" // optional; default APP_ORIGIN + /api/v1/integrations/gmail/callback
	EnvGmailPollSeconds   = "GMAIL_POLL_SECONDS"
	EnvGmailRefreshToken  = "GMAIL_REFRESH_TOKEN"   // written by cmd/gmail-auth; keeps the mailbox connected on every start
	EnvGoogleOAuthBaseURL = "GOOGLE_OAUTH_BASE_URL" // tests point these at a fake
	EnvGmailAPIBaseURL    = "GMAIL_API_BASE_URL"
)

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

// minSecretBytes is the shortest secret accepted for token encryption and webhook signing.
const minSecretBytes = 32

// defaultBrowserAPIPrefix matches the rewrite in apps/web/next.config.ts.
const defaultBrowserAPIPrefix = "/api"

// maxProxyHops is a sanity bound; real deployments have one or two.
const maxProxyHops = 5

type Config struct {
	Port             string
	DatabaseURL      string
	AppOrigin        string   // the web app; cookies and CSRF origin checks are tied to it
	NativeAppOrigins []string // Capacitor shells allowed by CORS
	JWESecret        []byte
	PaymentSecret    []byte
	AI               AI
	Gmail            Gmail
	MigrateOnStart   bool
	Environment      Environment
	CookieSecure     bool
	LogLevel         string
	RequestTimeout   time.Duration
	// BrowserAPIPrefix is the path browsers put in front of /v1: the web app proxies /api/*
	// here, so cookies scoped to a path must use the path the browser sees.
	BrowserAPIPrefix string
	FXBaseURL        string
	// WorkerInProcess runs the background jobs inside the API process. Production runs
	// cmd/worker separately so the API can scale to zero.
	WorkerInProcess bool
	// RateLimits turns the gateway's limits on. Tests leave it off unless they test it.
	RateLimits bool
	// TrustedProxyHops is how many proxies we run in front of the API; the caller's IP is that
	// many entries from the right of X-Forwarded-For, so a client can't forge its address.
	TrustedProxyHops int
	// PIIKey encrypts personal data at rest (package pii). Losing it loses that data, and
	// changing it needs a re-encryption, so production must set its own and keep it safe.
	PIIKey []byte
	// AnalyticsCacheTTL keeps computed analytics reports; zero turns the cache off (tests).
	AnalyticsCacheTTL time.Duration
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
		MigrateOnStart: get(EnvMigrateOnStart, "true") == "true",
		Environment:    Environment(get(EnvEnvironment, string(EnvDevelopment))),
		LogLevel:       get(EnvLogLevel, "info"),
		FXBaseURL:      os.Getenv(EnvFXBaseURL),
	}
	// LookupEnv, not get: an empty value is meaningful (the API is reached directly).
	c.BrowserAPIPrefix = defaultBrowserAPIPrefix
	if v, ok := os.LookupEnv(EnvBrowserAPIPrefix); ok {
		c.BrowserAPIPrefix = strings.TrimRight(v, "/")
	}
	c.CookieSecure = get(EnvCookieSecure, strconv.FormatBool(c.IsProduction())) == "true"
	c.WorkerInProcess = get(EnvWorkerInProcess, strconv.FormatBool(!c.IsProduction())) == "true"
	c.RateLimits = get(EnvRateLimits, "on") != "off"
	hops, err := strconv.Atoi(get(EnvTrustedProxyHops, "1"))
	if err != nil || hops < 0 || hops > maxProxyHops {
		return c, fmt.Errorf("%s must be a whole number from 0 to %d", EnvTrustedProxyHops, maxProxyHops)
	}
	c.TrustedProxyHops = hops
	c.PIIKey = PIIKey(os.Getenv(EnvPIIKey), c.JWESecret)
	cacheSecs, err := strconv.Atoi(get(EnvAnalyticsCacheSec, "600"))
	if err != nil || cacheSecs < 0 {
		return c, fmt.Errorf("%s must be a whole number of seconds, 0 or more", EnvAnalyticsCacheSec)
	}
	c.AnalyticsCacheTTL = time.Duration(cacheSecs) * time.Second
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
	poll, err := strconv.Atoi(get(EnvGmailPollSeconds, "5"))
	if err != nil || poll < 5 {
		return c, fmt.Errorf("%s must be 5 seconds or more", EnvGmailPollSeconds)
	}
	c.Gmail = Gmail{
		ClientID:     os.Getenv(EnvGoogleClientID),
		ClientSecret: os.Getenv(EnvGoogleClientSecret),
		RedirectURL:  get(EnvGmailRedirectURL, c.AppOrigin+"/api/v1/integrations/gmail/callback"),
		PollEvery:    time.Duration(poll) * time.Second,
		RefreshToken: os.Getenv(EnvGmailRefreshToken),
		OAuthBaseURL: os.Getenv(EnvGoogleOAuthBaseURL),
		APIBaseURL:   os.Getenv(EnvGmailAPIBaseURL),
	}

	var problems []string
	var aiProblems []string
	c.AI, aiProblems = LoadAI()
	problems = append(problems, aiProblems...)
	if c.DatabaseURL == "" {
		problems = append(problems, EnvDatabaseURL+" is required")
	}
	if len(c.JWESecret) < minSecretBytes {
		problems = append(problems, fmt.Sprintf("%s must be at least %d bytes", EnvJWESecret, minSecretBytes))
	}
	if len(c.PaymentSecret) < minSecretBytes {
		problems = append(problems, fmt.Sprintf("%s must be at least %d bytes", EnvPaymentSecret, minSecretBytes))
	}
	if c.IsProduction() && os.Getenv(EnvPIIKey) == "" {
		problems = append(problems, EnvPIIKey+" is required in production (it encrypts guests' personal data)")
	}
	if len(c.PIIKey) < minSecretBytes {
		problems = append(problems, fmt.Sprintf("%s must be at least %d bytes", EnvPIIKey, minSecretBytes))
	}
	if c.BrowserAPIPrefix != "" && !strings.HasPrefix(c.BrowserAPIPrefix, "/") {
		problems = append(problems, EnvBrowserAPIPrefix+" must be empty or start with /")
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

// Gmail is the OAuth client the admin connects a mailbox with. Without a client ID the
// inbox works as before: replies are simulated in the outbox.
type Gmail struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	PollEvery    time.Duration
	RefreshToken string // the mailbox grant from cmd/gmail-auth; empty: whatever the database holds
	OAuthBaseURL string // empty: Google's
	APIBaseURL   string // empty: Google's
}

func (g Gmail) Configured() bool { return g.ClientID != "" && g.ClientSecret != "" }

// AI configures the model chain. With no keys, enquiries use the keyword rules and templates.
type AI struct {
	GeminiKey     string
	GeminiModels  []string // empty: discover
	GeminiBaseURL string   // empty: Google's endpoint; tests point it at a fake
	GeminiRPM     int      // per model; 0 = no pacing
	GroqKey       string
	GroqModels    []string
	GroqBaseURL   string
	GroqRPM       int
}

func (a AI) Enabled() bool { return a.GeminiKey != "" || a.GroqKey != "" }

// LoadAI reads the AI settings; the eval command uses it without the rest of the config.
func LoadAI() (AI, []string) {
	a := AI{
		GeminiKey: os.Getenv(EnvGeminiAPIKey), GeminiModels: list(os.Getenv(EnvGeminiModels)), GeminiBaseURL: os.Getenv(EnvGeminiBaseURL),
		GroqKey: os.Getenv(EnvGroqAPIKey), GroqModels: list(os.Getenv(EnvGroqModels)), GroqBaseURL: os.Getenv(EnvGroqBaseURL),
	}
	var problems []string
	rpm := func(env, fallback string) int {
		n, err := strconv.Atoi(get(env, fallback))
		if err != nil || n < 0 {
			problems = append(problems, env+" must be a whole number, 0 or more")
		}
		return n
	}
	// Free-tier limits: about 10 requests a minute per Gemini model, 30 per Groq model.
	a.GeminiRPM, a.GroqRPM = rpm(EnvGeminiRPM, "10"), rpm(EnvGroqRPM, "30")
	return a, problems
}

func list(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// PIIKey is the key for personal data: PII_KEY when set; otherwise, for development only, one
// derived from the token secret so a local run needs no extra setup. The seeder, the API and
// the tests all call this, so they always agree on the key.
func PIIKey(explicit string, jweSecret []byte) []byte {
	if explicit != "" {
		return []byte(explicit)
	}
	m := hmac.New(sha256.New, jweSecret)
	m.Write([]byte("kaapi/pii/development-key"))
	return m.Sum(nil)
}
