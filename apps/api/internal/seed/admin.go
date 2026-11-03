package seed

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
)

// minAdminPassword is the shortest administrator password the seeder accepts.
const minAdminPassword = 8

// Account is the deployment's own administrator. It comes from the environment
// (SEED_ADMIN_*), never from the fixtures, so a real email and password stay out of git.
type Account struct {
	Email    string
	Name     string
	Password string
}

func (a Account) validate() error {
	if addr, err := mail.ParseAddress(a.Email); err != nil || addr.Address != a.Email {
		return errors.New("seed: SEED_ADMIN_EMAIL is not a valid email address")
	}
	if len(a.Password) < minAdminPassword {
		return errors.New("seed: SEED_ADMIN_PASSWORD must be at least 8 characters")
	}
	return nil
}

// EnsureAdmin creates the administrator, or resets an existing account with that email to
// ADMIN with the given password. On a reset its sessions are revoked, so an old password's
// sign-ins stop working at once.
func EnsureAdmin(ctx context.Context, pool *pgxpool.Pool, a Account) error {
	return db.WithTx(ctx, pool, func(tx pgx.Tx) error { return ensureAdmin(ctx, tx, a) })
}

func ensureAdmin(ctx context.Context, tx pgx.Tx, a Account) error {
	a.Email = strings.ToLower(strings.TrimSpace(a.Email))
	if a.Name = strings.TrimSpace(a.Name); a.Name == "" {
		a.Name = "Kaapi Admin"
	}
	if err := a.validate(); err != nil {
		return err
	}
	hash, err := auth.HashPassword(a.Password)
	if err != nil {
		return err
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO users (id, email, name, role, password_hash) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT ((lower(email))) DO UPDATE SET name = EXCLUDED.name, role = EXCLUDED.role,
			password_hash = EXCLUDED.password_hash, is_active = true, updated_at = now()
		RETURNING id`, newID(), a.Email, a.Name, domain.RoleAdmin, hash).Scan(&id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id)
	return err
}
