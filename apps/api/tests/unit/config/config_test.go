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

func TestStaffPasswordOnlyDefaultsToTheReviewerAccount(t *testing.T) {
	t.Setenv(config.EnvDatabaseURL, "postgres://unused")
	t.Setenv(config.EnvJWESecret, "0123456789abcdef0123456789abcdef")
	t.Setenv(config.EnvPaymentSecret, "0123456789abcdef0123456789abcdef")

	os.Unsetenv(config.EnvStaffPasswordOnly)
	if c, _ := config.Load(); len(c.StaffPasswordOnly) != 1 || c.StaffPasswordOnly[0] != "admin.kaapi@gmail.com" {
		t.Fatalf("default: %q", c.StaffPasswordOnly)
	}
	t.Setenv(config.EnvStaffPasswordOnly, "") // set but empty: everyone uses the emailed code
	if c, _ := config.Load(); len(c.StaffPasswordOnly) != 0 {
		t.Fatalf("empty: %q", c.StaffPasswordOnly)
	}
	t.Setenv(config.EnvStaffPasswordOnly, " A@x.in , b@y.in ")
	if c, _ := config.Load(); len(c.StaffPasswordOnly) != 2 || c.StaffPasswordOnly[0] != "a@x.in" {
		t.Fatalf("list: %q", c.StaffPasswordOnly)
	}
}
