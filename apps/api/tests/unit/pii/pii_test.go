package pii_test

import (
	"errors"
	"strings"
	"testing"

	"kaapi/api/internal/platform/pii"
)

const keyA, keyB = "unit-test-pii-key-A-0123456789abcdef", "unit-test-pii-key-B-0123456789abcdef"

func configure(t *testing.T, key string) {
	t.Helper()
	if err := pii.Configure([]byte(key)); err != nil {
		t.Fatal(err)
	}
}

func TestSealOpenRoundTripWithARandomNonce(t *testing.T) {
	configure(t, keyA)
	a, err := pii.Seal("guest@example.test")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := pii.Seal("guest@example.test")
	if a == b || !pii.IsSealed(a) || strings.Contains(a, "guest") {
		t.Fatalf("same value must encrypt differently each time, and never show through: %q %q", a, b)
	}
	if got, err := pii.Open(a); err != nil || got != "guest@example.test" {
		t.Fatalf("open: %q %v", got, err)
	}
	if s, _ := pii.Seal(""); s != "" {
		t.Fatal("an empty value stays empty")
	}
}

func TestTamperedOrWrongKeyCiphertextIsRefused(t *testing.T) {
	configure(t, keyA)
	sealed, _ := pii.Seal("+91 90000 20001")
	flipped := []byte(sealed)
	last := len(flipped) - 2
	flipped[last] ^= 0x01
	if _, err := pii.Open(string(flipped)); !errors.Is(err, pii.ErrBadCiphertext) {
		t.Fatalf("tampered ciphertext must fail (GCM authenticates it), got %v", err)
	}
	configure(t, keyB)
	if _, err := pii.Open(sealed); !errors.Is(err, pii.ErrBadCiphertext) {
		t.Fatalf("another key must not decrypt it, got %v", err)
	}
}

func TestPlaintextFromBeforeEncryptionPassesThrough(t *testing.T) {
	configure(t, keyA)
	if got, err := pii.Open("Meera Iyer"); err != nil || got != "Meera Iyer" {
		t.Fatalf("legacy plaintext: %q %v", got, err)
	}
}

func TestBlindIndexIsStableNormalisedAndKeyed(t *testing.T) {
	configure(t, keyA)
	a := pii.EmailIndex("Guest@Example.test ")
	if a != pii.EmailIndex("guest@example.test") {
		t.Fatal("emails match case-insensitively, ignoring spaces")
	}
	if a == pii.Index("phone", "guest@example.test") {
		t.Fatal("different kinds of identifier must not share hashes")
	}
	if strings.Contains(a, "guest") {
		t.Fatal("the index must not reveal the value")
	}
	configure(t, keyB)
	if a == pii.EmailIndex("guest@example.test") {
		t.Fatal("the index is keyed: without the key it can't be recomputed")
	}
}
