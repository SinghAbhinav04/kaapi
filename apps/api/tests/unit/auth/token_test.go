package auth_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
)

var secret = []byte("test-only-jwe-secret-0123456789abcdef")

func TestSealerRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	s := auth.NewSealer(secret, func() time.Time { return now })
	c := auth.Claims{UserID: uuid.New(), Role: domain.RoleGM, SessionID: uuid.New(), IssuedAt: now.Unix(), ExpiresAt: now.Add(auth.AccessTTL).Unix()}
	tok, err := s.Seal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(tok, ".") != 4 {
		t.Fatalf("not a compact JWE: %s", tok)
	}
	got, err := s.Open(tok)
	if err != nil || got != c {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestSealerRejects(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	s := auth.NewSealer(secret, func() time.Time { return now })
	good, _ := s.Seal(auth.Claims{UserID: uuid.New(), Role: domain.RoleGuest, ExpiresAt: now.Add(time.Minute).Unix()})
	expired, _ := s.Seal(auth.Claims{UserID: uuid.New(), Role: domain.RoleGuest, ExpiresAt: now.Add(-time.Second).Unix()})
	badRole, _ := s.Seal(auth.Claims{UserID: uuid.New(), Role: "ROOT", ExpiresAt: now.Add(time.Minute).Unix()})
	other, _ := auth.NewSealer([]byte("another-secret-entirely-0123456789ab"), func() time.Time { return now }).
		Seal(auth.Claims{UserID: uuid.New(), Role: domain.RoleGM, ExpiresAt: now.Add(time.Minute).Unix()})
	tampered := good[:len(good)-4] + "AAAA"

	for name, tok := range map[string]string{"expired": expired, "unknown role": badRole, "other key": other,
		"tampered": tampered, "garbage": "not-a-token"} {
		if _, err := s.Open(tok); !errors.Is(err, auth.ErrTokenInvalid) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
