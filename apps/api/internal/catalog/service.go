// Package catalog reads what the resort offers: room types, photos, rate plans, the
// nightly price calendar and the property facts used by the site and the reply drafts.
package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Rooms returns every active room type with its photos and plans, in display order.
func (s *Service) Rooms(ctx context.Context) ([]RoomType, error) {
	rooms, err := s.store.activeRooms(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	return rooms, s.store.attach(ctx, s.pool, rooms)
}

// Room returns one room type by slug.
func (s *Service) Room(ctx context.Context, slug string) (RoomType, error) {
	room, err := s.store.roomBySlug(ctx, s.pool, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoomType{}, ErrRoomNotFound
	}
	if err != nil {
		return RoomType{}, err
	}
	list := []RoomType{room}
	if err := s.store.attach(ctx, s.pool, list); err != nil {
		return RoomType{}, err
	}
	return list[0], nil
}

// Calendar returns nightly prices and rooms left for one room type and plan.
func (s *Service) Calendar(ctx context.Context, slug, planCode string, from, to time.Time) ([]CalendarDay, error) {
	if n := domain.NightsBetween(from, to); n < 1 || n > maxCalendarDays {
		return nil, ErrBadRange
	}
	planID, err := s.store.planID(ctx, s.pool, slug, planCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.store.calendar(ctx, s.pool, planID, from, to)
}

// Property returns the resort's basic settings and its fact sheet.
func (s *Service) Property(ctx context.Context) (Property, error) {
	return s.store.property(ctx, s.pool)
}
