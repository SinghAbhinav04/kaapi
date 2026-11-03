package gmail

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Separate keys for separate jobs, both derived from the server secret.
func deriveKey(secret []byte, purpose string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("kaapi/gmail/" + purpose))
	return m.Sum(nil)
}

func seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func open(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("gmail: sealed token too short")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}

// signState binds the OAuth round trip to the admin who started it, for stateTTL.
func signState(key []byte, user uuid.UUID, expires time.Time) string {
	payload := user.String() + "." + strconv.FormatInt(expires.Unix(), 10)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func verifyState(key []byte, state string, now time.Time) (uuid.UUID, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		return uuid.Nil, ErrBadState
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(parts[0] + "." + parts[1]))
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(got, m.Sum(nil)) {
		return uuid.Nil, ErrBadState
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.After(time.Unix(exp, 0)) {
		return uuid.Nil, ErrBadState
	}
	user, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, ErrBadState
	}
	return user, nil
}
