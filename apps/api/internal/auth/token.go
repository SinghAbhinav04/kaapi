package auth

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// Sealer encrypts and decrypts access tokens (JWE, direct key, AES-256-GCM).
// Encrypted rather than only signed, so even the role and ids aren't readable in transit
// logs or browser storage.
type Sealer struct {
	key []byte
	now func() time.Time
}

// NewSealer derives the 256-bit key from the configured secret.
func NewSealer(secret []byte, now func() time.Time) *Sealer {
	k := sha256.Sum256(secret)
	if now == nil {
		now = time.Now
	}
	return &Sealer{key: k[:], now: now}
}

func (s *Sealer) Seal(c Claims) (string, error) {
	enc, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: s.key},
		(&jose.EncrypterOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	obj, err := enc.Encrypt(payload)
	if err != nil {
		return "", err
	}
	return obj.CompactSerialize()
}

// Open decrypts a token and checks it hasn't expired. Every failure is ErrTokenInvalid:
// callers don't need to know whether it was tampered with or merely old.
func (s *Sealer) Open(token string) (Claims, error) {
	obj, err := jose.ParseEncrypted(token, []jose.KeyAlgorithm{jose.DIRECT}, []jose.ContentEncryption{jose.A256GCM})
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}
	payload, err := obj.Decrypt(s.key)
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrTokenInvalid
	}
	if s.now().Unix() >= c.ExpiresAt || !c.Role.Valid() {
		return Claims{}, ErrTokenInvalid
	}
	return c, nil
}

// issue builds claims for a session.
func (s *Sealer) issue(u User, sessionID [16]byte) (string, time.Time, error) {
	now := s.now()
	exp := now.Add(AccessTTL)
	tok, err := s.Seal(Claims{UserID: u.ID, Role: u.Role, SessionID: sessionID, IssuedAt: now.Unix(), ExpiresAt: exp.Unix()})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("seal access token: %w", err)
	}
	return tok, exp, nil
}
