package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
)

// store is every query this package runs. Emails and names are encrypted at rest (package
// pii); an account is found by its email's blind index.
type store struct{}

// account is a user row as sign-in needs it.
type account struct {
	User
	hash   *string
	active bool
}

// sessionRow is a session found by its refresh token, with its user.
type sessionRow struct {
	id         uuid.UUID
	expired    bool
	revokedAt  *time.Time
	replacedBy *uuid.UUID
	user       User
	active     bool
}

// accountByEmail returns pgx.ErrNoRows when nobody has that email.
func (store) accountByEmail(ctx context.Context, q db.Querier, email string) (account, error) {
	var a account
	err := q.QueryRow(ctx, `SELECT id, email, name, role, password_hash, is_active FROM users WHERE email_hash = $1`, pii.EmailIndex(email)).
		Scan(&a.ID, pii.Into(&a.Email), pii.Into(&a.Name), &a.Role, &a.hash, &a.active)
	return a, err
}

// activeUser returns pgx.ErrNoRows for an unknown or deactivated account.
func (store) activeUser(ctx context.Context, q db.Querier, id uuid.UUID) (User, error) {
	var u User
	err := q.QueryRow(ctx, `SELECT id, email, name, role FROM users WHERE id = $1 AND is_active`, id).
		Scan(&u.ID, pii.Into(&u.Email), pii.Into(&u.Name), &u.Role)
	return u, err
}

func (store) insertGuest(ctx context.Context, q db.Querier, u User) error {
	_, err := q.Exec(ctx, `INSERT INTO users (id, email, email_hash, name, role) VALUES ($1,$2,$3,$4,$5)`,
		u.ID, pii.Text(u.Email), pii.EmailIndex(u.Email), pii.Text(u.Name), u.Role)
	return err
}

// queueEmail puts an email in the outbox for the worker to send through the connected mailbox.
func (store) queueEmail(ctx context.Context, q db.Querier, kind domain.OutboxKind, to, subject, body string) error {
	_, err := q.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, status, recipient, subject, body) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		idgen.New(), domain.OutboxEmail, kind, domain.OutboxQueued, pii.Text(to), subject, pii.Text(body))
	return err
}

// ---- one-time codes (only the email's blind index is kept: a code is only ever matched)

func (store) recentOTPs(ctx context.Context, q db.Querier, email string, window time.Duration) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM otp_codes WHERE email_hash = $1 AND created_at > now() - make_interval(secs => $2)`,
		pii.EmailIndex(email), window.Seconds()).Scan(&n)
	return n, err
}

func (store) insertOTP(ctx context.Context, q db.Querier, email string, codeHash []byte, ttl time.Duration) error {
	_, err := q.Exec(ctx, `INSERT INTO otp_codes (id, email_hash, purpose, code_hash, expires_at)
		VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5))`,
		idgen.New(), pii.EmailIndex(email), domain.OTPGuestLogin, codeHash, ttl.Seconds())
	return err
}

// latestOTP locks the newest live code for an email; pgx.ErrNoRows when there is none.
func (store) latestOTP(ctx context.Context, q db.Querier, email string) (id uuid.UUID, codeHash []byte, attempts int, err error) {
	err = q.QueryRow(ctx, `SELECT id, code_hash, attempts FROM otp_codes
		WHERE email_hash = $1 AND purpose = $2 AND consumed_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, pii.EmailIndex(email), domain.OTPGuestLogin).Scan(&id, &codeHash, &attempts)
	return id, codeHash, attempts, err
}

func (store) countOTPAttempt(ctx context.Context, q db.Querier, id uuid.UUID, ceiling int) error {
	_, err := q.Exec(ctx, `UPDATE otp_codes SET attempts = least(attempts + 1, $2) WHERE id = $1`, id, ceiling)
	return err
}

func (store) consumeOTP(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE otp_codes SET consumed_at = now() WHERE id = $1`, id)
	return err
}

// ---- sessions

func (store) insertSession(ctx context.Context, q db.Querier, id, userID uuid.UUID, refreshHash []byte, userAgent string, ipHash []byte, expires time.Time) error {
	_, err := q.Exec(ctx, `INSERT INTO sessions (id, user_id, refresh_hash, user_agent, ip_hash, expires_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, userID, refreshHash, userAgent, ipHash, expires)
	return err
}

// sessionByRefresh locks the session a refresh token belongs to; pgx.ErrNoRows if none.
func (store) sessionByRefresh(ctx context.Context, q db.Querier, refreshHash []byte) (sessionRow, error) {
	var r sessionRow
	err := q.QueryRow(ctx, `SELECT s.id, s.expires_at <= now(), s.revoked_at, s.replaced_by, u.id, u.email, u.name, u.role, u.is_active
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.refresh_hash = $1 FOR UPDATE OF s`, refreshHash).
		Scan(&r.id, &r.expired, &r.revokedAt, &r.replacedBy, &r.user.ID, pii.Into(&r.user.Email), pii.Into(&r.user.Name), &r.user.Role, &r.active)
	return r, err
}

func (store) replaceSession(ctx context.Context, q db.Querier, id, by uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now(), replaced_by = $2 WHERE id = $1`, id, by)
	return err
}

func (store) revokeSession(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

func (store) revokeAllSessions(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// sessionLive: not revoked, not expired, and its user still active.
func (store) sessionLive(ctx context.Context, q db.Querier, id uuid.UUID) (bool, error) {
	var live bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sessions se JOIN users u ON u.id = se.user_id
		WHERE se.id = $1 AND se.revoked_at IS NULL AND se.expires_at > now() AND u.is_active)`, id).Scan(&live)
	return live, err
}
