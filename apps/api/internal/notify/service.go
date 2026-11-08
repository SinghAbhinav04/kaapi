// Package notify is the console's alerts: complaints and enquiries needing sorting for the
// GM, late payments needing a refund. Other packages write them (in the same transaction as
// what caused them); this one reads them and marks them read. An alert is for a team, so one
// person reading it clears it for the team.
package notify

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// List returns the newest alerts the viewer may see and the number still unread.
func (s *Service) List(ctx context.Context, v Viewer, limit int) (Page, error) {
	if limit <= 0 || limit > MaxLimit {
		limit = DefaultLimit
	}
	items, err := s.store.list(ctx, s.pool, v, limit)
	if err != nil {
		return Page{}, err
	}
	unread, err := s.store.unread(ctx, s.pool, v)
	return Page{Items: items, Unread: unread}, err
}

// MarkRead marks one alert read. One the viewer can't see doesn't exist for them.
func (s *Service) MarkRead(ctx context.Context, v Viewer, id uuid.UUID) error {
	n, err := s.store.markRead(ctx, s.pool, v, &id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkAllRead marks every alert the viewer can see read.
func (s *Service) MarkAllRead(ctx context.Context, v Viewer) error {
	_, err := s.store.markRead(ctx, s.pool, v, nil)
	return err
}
