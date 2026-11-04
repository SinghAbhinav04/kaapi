package gmail

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/platform/db"
)

// store is every query this package runs: the one integrations row for the mailbox.
type store struct{}

// load returns ErrNotConnected when no mailbox is connected.
func (store) load(ctx context.Context, q db.Querier) (stored, error) {
	var r stored
	var history *string
	err := q.QueryRow(ctx, `SELECT account_email, refresh_token_enc, history_id, connected_at, last_sync_at, last_error
		FROM integrations WHERE provider = $1`, provider).Scan(&r.email, &r.sealed, &history, &r.connectedAt, &r.lastSync, &r.lastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotConnected
	}
	if history != nil {
		r.historyID = *history
	}
	return r, err
}

func (store) exists(ctx context.Context, q db.Querier) bool {
	var ok bool
	_ = q.QueryRow(ctx, `SELECT true FROM integrations WHERE provider = $1`, provider).Scan(&ok)
	return ok
}

// upsert saves a (new) connection and resets its sync state.
func (store) upsert(ctx context.Context, q db.Querier, email string, sealed []byte, historyID string, user *uuid.UUID, at time.Time) error {
	_, err := q.Exec(ctx, `INSERT INTO integrations (provider, account_email, refresh_token_enc, history_id, connected_by, connected_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider) DO UPDATE SET account_email = $2, refresh_token_enc = $3, history_id = $4, connected_by = $5,
			connected_at = $6, last_sync_at = NULL, last_error = NULL`,
		provider, email, sealed, historyID, user, at)
	return err
}

func (store) remove(ctx context.Context, q db.Querier) error {
	_, err := q.Exec(ctx, `DELETE FROM integrations WHERE provider = $1`, provider)
	return err
}

func (store) setError(ctx context.Context, q db.Querier, msg string) {
	_, _ = q.Exec(ctx, `UPDATE integrations SET last_error = $2 WHERE provider = $1`, provider, msg)
}

// synced records a good poll; historyID (when not nil) is where the next one starts.
func (store) synced(ctx context.Context, q db.Querier, at time.Time, historyID *string) error {
	_, err := q.Exec(ctx, `UPDATE integrations SET last_sync_at = $2, last_error = NULL, history_id = coalesce($3, history_id)
		WHERE provider = $1`, provider, at, historyID)
	return err
}
