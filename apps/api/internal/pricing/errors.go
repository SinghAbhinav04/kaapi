package pricing

import "errors"

var (
	ErrNoNights     = errors.New("pricing: at least one night is required")
	ErrNoRooms      = errors.New("pricing: at least one room is required")
	ErrTooManyGuest = errors.New("pricing: too many guests for the rooms chosen")
	ErrBadSlabs     = errors.New("pricing: GST slabs must end with an open band")
)

// Promo eligibility. The messages are shown to guests as written.
var (
	ErrPromoInactive  = errors.New("this code is not active")
	ErrPromoNotYet    = errors.New("this code can't be used yet")
	ErrPromoExpired   = errors.New("this code has expired")
	ErrPromoStayDates = errors.New("this code doesn't cover your dates")
	ErrPromoMinNights = errors.New("this code needs a longer stay")
	ErrPromoRoomType  = errors.New("this code doesn't apply to this room")
	ErrPromoUsedUp    = errors.New("this code has been fully used")
)
