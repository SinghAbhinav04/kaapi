// Package inventory owns what the website may sell: holds, confirmations, availability
// and allotment changes. Every path that changes inventory locks the night rows first,
// always in date order, so two overlapping stays can never deadlock or oversell.
package inventory

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time // wall clock for "today" checks; hold expiry always uses the database clock
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, now: now}
}

// today is the resort's current date, as a midnight-UTC calendar date like every stay date.
func (s *Service) today(st settings) time.Time {
	t := s.now().In(st.location)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
