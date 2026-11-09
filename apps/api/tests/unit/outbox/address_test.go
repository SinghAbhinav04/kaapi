package outbox_test

import (
	"testing"

	"kaapi/api/internal/domain"
)

func TestReservedAddressesAreNeverSent(t *testing.T) {
	for addr, want := range map[string]bool{
		"guest@example.test":               true,
		"erased+KS-ABCDEF@example.invalid": true,
		"someone@EXAMPLE.com":              true,
		"a@mail.example.org":               true,
		"no-at-sign":                       true,
		"resorts.kaapi@gmail.com":          false,
		"asha@company.in":                  false,
		"me@test.com":                      false,
	} {
		if got := domain.ReservedAddress(addr); got != want {
			t.Errorf("Reserved(%q) = %v, want %v", addr, got, want)
		}
	}
}
