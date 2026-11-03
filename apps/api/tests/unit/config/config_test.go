package config_test

import (
	"os"
	"testing"

	"kaapi/api/internal/platform/config"
)

func TestBrowserAPIPrefixDefaultsToTheWebProxy(t *testing.T) {
	t.Setenv(config.EnvDatabaseURL, "postgres://unused")
	t.Setenv(config.EnvJWESecret, "0123456789abcdef0123456789abcdef")
	t.Setenv(config.EnvPaymentSecret, "0123456789abcdef0123456789abcdef")

	os.Unsetenv(config.EnvBrowserAPIPrefix)
	if c, err := config.Load(); err != nil || c.BrowserAPIPrefix != "/api" {
		t.Fatalf("default: %q %v", c.BrowserAPIPrefix, err)
	}
	t.Setenv(config.EnvBrowserAPIPrefix, "") // set but empty: the API is reached directly
	if c, err := config.Load(); err != nil || c.BrowserAPIPrefix != "" {
		t.Fatalf("empty: %q %v", c.BrowserAPIPrefix, err)
	}
	t.Setenv(config.EnvBrowserAPIPrefix, "api")
	if _, err := config.Load(); err == nil {
		t.Fatal("a prefix without a leading slash must be refused")
	}
}
