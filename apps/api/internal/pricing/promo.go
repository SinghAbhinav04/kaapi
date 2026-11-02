package pricing

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// PromoRule is a promo code as stored, with the limits that decide whether it applies.
type PromoRule struct {
	Promo
	StayFrom, StayTo time.Time
	BookFrom, BookTo time.Time
	MinNights        int
	MaxUses          *int
	UsedCount        int
	RoomTypeIDs      []uuid.UUID // empty = any room type
	Active           bool
}

var (
	ErrPromoInactive  = errors.New("this code is not active")
	ErrPromoNotYet    = errors.New("this code can't be used yet")
	ErrPromoExpired   = errors.New("this code has expired")
	ErrPromoStayDates = errors.New("this code doesn't cover your dates")
	ErrPromoMinNights = errors.New("this code needs a longer stay")
	ErrPromoRoomType  = errors.New("this code doesn't apply to this room")
	ErrPromoUsedUp    = errors.New("this code has been fully used")
)

// Eligible reports why a promo can't be used for this stay, or nil if it can.
// checkIn/checkOut are stay dates; bookedOn is today's date in the resort's time zone.
func Eligible(r PromoRule, checkIn, checkOut, bookedOn time.Time, roomTypeID uuid.UUID) error {
	lastNight := checkOut.AddDate(0, 0, -1)
	nights := int(checkOut.Sub(checkIn).Hours() / 24)
	switch {
	case !r.Active:
		return ErrPromoInactive
	case bookedOn.Before(r.BookFrom):
		return ErrPromoNotYet
	case bookedOn.After(r.BookTo):
		return ErrPromoExpired
	case checkIn.Before(r.StayFrom) || lastNight.After(r.StayTo):
		return ErrPromoStayDates
	case nights < r.MinNights:
		return ErrPromoMinNights
	case r.MaxUses != nil && r.UsedCount >= *r.MaxUses:
		return ErrPromoUsedUp
	}
	if len(r.RoomTypeIDs) > 0 {
		for _, id := range r.RoomTypeIDs {
			if id == roomTypeID {
				return nil
			}
		}
		return ErrPromoRoomType
	}
	return nil
}
