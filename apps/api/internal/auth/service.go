package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

type Service struct {
	pool   *pgxpool.Pool
	store  store
	sealer *Sealer
	key    []byte // HMAC key for OTP codes and IP hashes, derived from the token secret
	now    func() time.Time

	mu       sync.Mutex
	sessions map[uuid.UUID]time.Time // session id → live until (a cached check)
}

func NewService(pool *pgxpool.Pool, secret []byte, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("kaapi/otp-and-ip"))
	return &Service{pool: pool, sealer: NewSealer(secret, now), key: mac.Sum(nil), now: now, sessions: map[uuid.UUID]time.Time{}}
}

// Sealer exposes the token sealer to the gateway middleware.
func (s *Service) Sealer() *Sealer { return s.sealer }

var ErrInvalidEmail = apperr.Validation(map[string]string{"email": "must be a valid email address"})

// dummyHash makes a login for an unknown email take as long as one for a real account,
// so response timing doesn't reveal which emails exist.
var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("not-a-real-password-just-for-timing")
	return h
})

// StaffLogin checks an email and password for a staff account.
func (s *Service) StaffLogin(ctx context.Context, email, password string, client ClientInfo) (User, Tokens, error) {
	a, err := s.store.accountByEmail(ctx, s.pool, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return User{}, Tokens{}, err
	}
	u, hash := a.User, a.hash
	if err != nil || hash == nil || !a.active || !u.Role.IsStaff() {
		_, _ = CheckPassword(dummyHash(), password)
		return User{}, Tokens{}, ErrBadCredentials
	}
	ok, err := CheckPassword(*hash, password)
	if err != nil || !ok {
		return User{}, Tokens{}, ErrBadCredentials
	}
	var t Tokens
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		t, _, err = s.newSession(ctx, tx, u, client)
		return err
	})
	return u, t, err
}

// RequestOTP creates a sign-in code for a guest and "emails" it (to the outbox).
// It answers the same way whether or not the email has an account. The code is
// returned so demo mode can show it on screen; production never sends it to the client.
func (s *Service) RequestOTP(ctx context.Context, email string) (string, error) {
	email, err := normaliseEmail(email)
	if err != nil {
		return "", err
	}
	code, err := randomDigits(OTPLength)
	if err != nil {
		return "", err
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		recent, err := s.store.recentOTPs(ctx, tx, email, OTPWindow)
		if err != nil {
			return err
		}
		if recent >= OTPMaxPerWindow {
			return ErrOTPRateLimited
		}
		if err := s.store.insertOTP(ctx, tx, email, s.otpHash(email, code), OTPTTL); err != nil {
			return err
		}
		// Queued with the code itself: the worker emails it through the connected mailbox.
		return s.store.queueEmail(ctx, tx, domain.OutboxOTP, email, "Your Kaapi Estate sign-in code",
			fmt.Sprintf("Your sign-in code is %s. It expires in %d minutes. If you didn't ask for it, you can ignore this email.",
				code, int(OTPTTL.Minutes())))
	})
	return code, err
}

// VerifyOTP signs a guest in with a code, creating their guest account on first use.
func (s *Service) VerifyOTP(ctx context.Context, email, code string, client ClientInfo) (User, Tokens, error) {
	email, err := normaliseEmail(email)
	if err != nil {
		return User{}, Tokens{}, err
	}
	var u User
	var t Tokens
	wrongCode := false
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, stored, attempts, err := s.store.latestOTP(ctx, tx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOTPInvalid
		}
		if err != nil {
			return err
		}
		if attempts >= OTPMaxAttempts || subtle.ConstantTimeCompare(stored, s.otpHash(email, strings.TrimSpace(code))) != 1 {
			// Count the attempt and commit it, then report the failure outside the transaction.
			wrongCode = true
			return s.store.countOTPAttempt(ctx, tx, id, OTPMaxAttempts)
		}
		if err := s.store.consumeOTP(ctx, tx, id); err != nil {
			return err
		}
		if u, err = s.guestFor(ctx, tx, email); err != nil {
			return err
		}
		t, _, err = s.newSession(ctx, tx, u, client)
		return err
	})
	if wrongCode {
		return User{}, Tokens{}, ErrOTPInvalid
	}
	return u, t, err
}

// guestFor finds or creates the guest account for an email. A staff email can't sign in
// this way: staff use their password, and a code must never grant staff access.
func (s *Service) guestFor(ctx context.Context, tx pgx.Tx, email string) (User, error) {
	a, err := s.store.accountByEmail(ctx, tx, email)
	switch {
	case err == nil && (a.Role != domain.RoleGuest || !a.active):
		return User{}, ErrOTPInvalid
	case err == nil:
		return a.User, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return User{}, err
	}
	u := User{ID: idgen.New(), Email: email, Name: strings.SplitN(email, "@", 2)[0], Role: domain.RoleGuest}
	if err := s.store.insertGuest(ctx, tx, u); err != nil {
		return u, err
	}
	// First sign-in is registration: welcome them in the same transaction, so it's sent exactly once.
	return u, s.store.queueEmail(ctx, tx, domain.OutboxWelcome, email, "Welcome to Kaapi Estate", welcomeBody)
}

// Refresh rotates a refresh token. Presenting one that was already rotated means it
// leaked (the real client has moved on), so every session of that user is revoked.
func (s *Service) Refresh(ctx context.Context, refreshToken string, client ClientInfo) (User, Tokens, error) {
	if refreshToken == "" {
		return User{}, Tokens{}, ErrTokenInvalid
	}
	sum := sha256.Sum256([]byte(refreshToken))
	var u User
	var t Tokens
	reused := false
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := s.store.sessionByRefresh(ctx, tx, sum[:])
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		u = row.user
		if row.revokedAt != nil {
			if row.replacedBy != nil {
				reused = true
				return s.store.revokeAllSessions(ctx, tx, u.ID)
			}
			return ErrTokenInvalid
		}
		if row.expired || !row.active {
			return ErrTokenInvalid
		}
		var newSID uuid.UUID
		if t, newSID, err = s.newSession(ctx, tx, u, client); err != nil {
			return err
		}
		s.forget(row.id) // the old access token dies with its session (harmless if the transaction retries)
		return s.store.replaceSession(ctx, tx, row.id, newSID)
	})
	if reused {
		s.mu.Lock()
		clear(s.sessions) // every session of that user was revoked; don't keep vouching for any
		s.mu.Unlock()
		return User{}, Tokens{}, ErrTokenInvalid
	}
	return u, t, err
}

// Logout ends a session. Its access token stops working at once in this process and within
// SessionCheckTTL in any other (SessionLive).
func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	s.forget(sessionID)
	return s.store.revokeSession(ctx, s.pool, sessionID)
}

// SessionLive reports whether an access token's session still stands: not revoked (logout,
// rotation, a detected token reuse), not expired, and its user still active. The token alone
// can't say this, since it is valid until it expires; the answer is cached briefly so the
// database sees one check per session every SessionCheckTTL, not one per request.
func (s *Service) SessionLive(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	now := s.now()
	s.mu.Lock()
	until, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if ok && now.Before(until) {
		return true, nil
	}
	live, err := s.store.sessionLive(ctx, s.pool, sessionID)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !live {
		delete(s.sessions, sessionID)
		return false, nil
	}
	if len(s.sessions) > maxCachedSessions { // bounded: drop finished entries, or all if still full
		for id, t := range s.sessions {
			if !now.Before(t) {
				delete(s.sessions, id)
			}
		}
		if len(s.sessions) > maxCachedSessions {
			clear(s.sessions)
		}
	}
	s.sessions[sessionID] = now.Add(SessionCheckTTL)
	return true, nil
}

func (s *Service) forget(ids ...uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		delete(s.sessions, id)
	}
}

// Me returns the signed-in user.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (User, error) {
	u, err := s.store.activeUser(ctx, s.pool, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrTokenInvalid
	}
	return u, err
}

func (s *Service) newSession(ctx context.Context, tx pgx.Tx, u User, client ClientInfo) (Tokens, uuid.UUID, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return Tokens{}, uuid.Nil, err
	}
	refresh := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(refresh))
	sid := idgen.New()
	refreshExp := s.now().Add(RefreshTTL)
	ua := client.UserAgent
	if len(ua) > userAgentMaxLen {
		ua = ua[:userAgentMaxLen]
	}
	if err := s.store.insertSession(ctx, tx, sid, u.ID, sum[:], ua, s.mac(client.IP), refreshExp); err != nil {
		return Tokens{}, uuid.Nil, err
	}
	access, accessExp, err := s.sealer.issue(u, sid)
	if err != nil {
		return Tokens{}, uuid.Nil, err
	}
	return Tokens{Access: access, AccessExpiresAt: accessExp, Refresh: refresh, RefreshExpiresAt: refreshExp}, sid, nil
}

// otpHash binds a code to its email under a server-side key, so a leaked table can't be
// brute-forced offline (there are only a million 6-digit codes).
func (s *Service) otpHash(email, code string) []byte { return s.mac(email + "|" + code) }

func (s *Service) mac(v string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(v))
	return m.Sum(nil)
}

func normaliseEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func randomDigits(n int) (string, error) {
	var b strings.Builder
	ten := big.NewInt(10)
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, ten)
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + d.Int64()))
	}
	return b.String(), nil
}
