package auth_test

import (
	"strings"
	"testing"

	"kaapi/api/internal/auth"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected format %q", hash)
	}
	if ok, err := auth.CheckPassword(hash, "correct horse"); err != nil || !ok {
		t.Fatalf("right password rejected: %v", err)
	}
	if ok, _ := auth.CheckPassword(hash, "correct horse "); ok {
		t.Fatal("wrong password accepted")
	}
	other, _ := auth.HashPassword("correct horse")
	if other == hash {
		t.Fatal("two hashes of the same password should differ (random salt)")
	}
}

func TestCheckPasswordRejectsMalformedHash(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$!!$!!", "$bcrypt$x$y$z$w"} {
		if _, err := auth.CheckPassword(h, "x"); err == nil {
			t.Errorf("%q: expected an error", h)
		}
	}
}
