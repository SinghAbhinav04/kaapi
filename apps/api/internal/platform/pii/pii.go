// Package pii encrypts personal data at the database boundary (DPDP: guests' details are
// unreadable at rest, in backups and to anyone with database access alone).
//
// Text is the interceptor. A store passes pii.Text as a query argument and it is encrypted
// on the way in (driver.Valuer); scanning into a *pii.Text decrypts on the way out
// (sql.Scanner). Domain code and handlers only ever see plain strings, so nothing above the
// store can forget to encrypt or leak ciphertext.
//
// Encryption is AES-256-GCM with a random nonce, so the same email encrypts differently every
// time and ciphertext can't be compared. Equality lookups (a guest's bookings, a sign-in
// email) use Index: a keyed hash stored in its own column.
package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
)

// Stored values look like "pii:v1:<base64(nonce | ciphertext+tag)>". The version names the
// key, so a new key can be introduced (v2) while v1 rows are still readable.
const (
	prefix     = "pii:"
	version    = "v1"
	sealedHead = prefix + version + ":"
	keyBytes   = 32
	encDomain  = "kaapi/pii/encrypt/" + version
	idxDomain  = "kaapi/pii/index/" + version
)

var (
	ErrNotConfigured = errors.New("pii: no key configured")
	ErrBadCiphertext = errors.New("pii: value can't be decrypted with this key")
)

type keys struct {
	aead  cipher.AEAD
	index []byte
}

var current atomic.Pointer[keys]

// Configure sets the key from a secret of at least 32 bytes; the encryption and index keys
// are derived from it separately. The app calls it once at start-up.
func Configure(secret []byte) error {
	if len(secret) < keyBytes {
		return fmt.Errorf("pii: key must be at least %d bytes", keyBytes)
	}
	block, err := aes.NewCipher(derive(secret, encDomain))
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	current.Store(&keys{aead: aead, index: derive(secret, idxDomain)})
	return nil
}

func derive(secret []byte, domain string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(domain))
	return m.Sum(nil)
}

// Seal encrypts s. The empty string stays empty (an absent phone is not a secret).
func Seal(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	k := current.Load()
	if k == nil {
		return "", ErrNotConfigured
	}
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := k.aead.Seal(nonce, nonce, []byte(s), []byte(sealedHead))
	return sealedHead + base64.RawStdEncoding.EncodeToString(out), nil
}

// Open decrypts a stored value. A value without the prefix is plaintext written before
// encryption existed; it is returned as is (the backfill encrypts it).
func Open(stored string) (string, error) {
	if !IsSealed(stored) {
		return stored, nil
	}
	k := current.Load()
	if k == nil {
		return "", ErrNotConfigured
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, sealedHead))
	n := k.aead.NonceSize()
	if err != nil || len(raw) < n {
		return "", ErrBadCiphertext
	}
	plain, err := k.aead.Open(nil, raw[:n], raw[n:], []byte(sealedHead))
	if err != nil {
		return "", ErrBadCiphertext
	}
	return string(plain), nil
}

// IsSealed reports whether a stored value is already encrypted.
func IsSealed(stored string) bool { return strings.HasPrefix(stored, sealedHead) }

// Index is a stable keyed hash of an identifier, for equality lookups on encrypted columns.
// kind keeps hashes of different things apart; emails are compared case-insensitively.
func Index(kind, value string) string {
	k := current.Load()
	if k == nil {
		panic(ErrNotConfigured) // a lookup without a key would silently match nothing
	}
	m := hmac.New(sha256.New, k.index)
	m.Write([]byte(kind + "|" + strings.ToLower(strings.TrimSpace(value))))
	return base64.RawStdEncoding.EncodeToString(m.Sum(nil))
}

// EmailIndex is Index for email addresses.
func EmailIndex(email string) string { return Index("email", email) }

// Text is a personal-data column: plain in Go, encrypted in the database.
type Text string

// Value encrypts on the way into the database.
func (t Text) Value() (driver.Value, error) { return Seal(string(t)) }

// Scan decrypts on the way out. NULL scans as the empty string.
func (t *Text) Scan(src any) error {
	var stored string
	switch v := src.(type) {
	case nil:
		*t = ""
		return nil
	case string:
		stored = v
	case []byte:
		stored = string(v)
	default:
		return fmt.Errorf("pii: can't scan %T", src)
	}
	plain, err := Open(stored)
	*t = Text(plain)
	return err
}

// NullText is a nullable personal-data column: nil stays NULL.
type NullText struct{ Text *string }

func (n NullText) Value() (driver.Value, error) {
	if n.Text == nil {
		return nil, nil
	}
	return Seal(*n.Text)
}

func (n *NullText) Scan(src any) error {
	if src == nil {
		n.Text = nil
		return nil
	}
	var t Text
	if err := t.Scan(src); err != nil {
		return err
	}
	s := string(t)
	n.Text = &s
	return nil
}

// Into decrypts a column straight into an existing string field: Scan(pii.Into(&b.Name)).
func Into(dst *string) *into { return &into{dst: dst} }

type into struct{ dst *string }

func (i *into) Scan(src any) error {
	var t Text
	err := t.Scan(src)
	*i.dst = string(t)
	return err
}

// IntoNull decrypts a nullable column straight into an existing *string field (nil for NULL).
func IntoNull(dst **string) *intoNull { return &intoNull{dst: dst} }

type intoNull struct{ dst **string }

func (i *intoNull) Scan(src any) error {
	var n NullText
	err := n.Scan(src)
	*i.dst = n.Text
	return err
}

// Null encrypts an optional value for a nullable column: nil stays NULL.
func Null(s *string) NullText { return NullText{Text: s} }
