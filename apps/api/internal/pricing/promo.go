package pricing

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
)

// Eligible reports why a promo can't be used for this stay, or nil if it can.
// bookedOn is today's date in the resort's time zone.
func Eligible(r PromoRule, stay domain.Stay, bookedOn time.Time, roomTypeID uuid.UUID) error {
	switch {
	case !r.Active:
		return ErrPromoInactive
	case bookedOn.Before(r.BookFrom):
		return ErrPromoNotYet
	case bookedOn.After(r.BookTo):
		return ErrPromoExpired
	case stay.CheckIn.Before(r.StayFrom) || stay.LastNight().After(r.StayTo):
		return ErrPromoStayDates
	case stay.Nights() < r.MinNights:
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
