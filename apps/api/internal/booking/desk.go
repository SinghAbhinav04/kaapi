package booking

import (
	"context"
	"time"

	"kaapi/api/internal/domain"
)

// Desk is the front desk's view of the days ahead: who arrives and leaves, how full tonight
// is, and refunds still owed.
type Desk struct {
	Today            time.Time
	Stays            []DeskStay // confirmed stays arriving or leaving in the window, by date
	SoldTonight      int        // website rooms sold for tonight
	AllotmentTonight int
	RefundsPending   int
}

// DeskStay is one confirmed stay as the desk needs it.
type DeskStay struct {
	Ref               string
	GuestName         string
	RoomType          string
	CheckIn, CheckOut time.Time
	Rooms             int
	Adults, Children  int
}

const (
	deskDays     = 7  // today and the six days after
	deskMaxStays = 60 // a busy week for 76 keys; more would not fit a glance anyway
)

// Desk reads the coming week for the staff's Today page.
func (s *Service) Desk(ctx context.Context) (Desk, error) {
	today := domain.ResortToday(s.now())
	d := Desk{Today: today}
	var err error
	if d.Stays, err = s.store.deskStays(ctx, s.pool, today, today.AddDate(0, 0, deskDays), deskMaxStays); err != nil {
		return d, err
	}
	err = s.store.deskCounts(ctx, s.pool, today, &d)
	return d, err
}
