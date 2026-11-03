package admin

import "kaapi/api/internal/platform/apperr"

var (
	ErrNothingToChange = apperr.New(apperr.CodeValidation, "Choose at least one thing to change.")
	ErrRoomNotFound    = apperr.New(apperr.CodeNotFound, "We couldn't find that room type.")
	ErrPlanNotFound    = apperr.New(apperr.CodeNotFound, "We couldn't find that rate plan.")
	ErrNoMatchingNight = apperr.New(apperr.CodeValidation, "No nights in that range fall on the chosen days.")
	ErrOfferNotFound   = apperr.New(apperr.CodeNotFound, "We couldn't find that offer.")
	ErrOfferCodeTaken  = apperr.New(apperr.CodeConflict, "Another offer already uses that code.")
	ErrOfferRoom       = apperr.Validation(map[string]string{"room_type_ids": "includes a room type we don't have"})
)
