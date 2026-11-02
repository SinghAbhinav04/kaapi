package admin

import "kaapi/api/internal/platform/apperr"

var (
	ErrNothingToChange = apperr.New(apperr.CodeValidation, "Choose at least one thing to change.")
	ErrRoomNotFound    = apperr.New(apperr.CodeNotFound, "We couldn't find that room type.")
	ErrPlanNotFound    = apperr.New(apperr.CodeNotFound, "We couldn't find that rate plan.")
	ErrNoMatchingNight = apperr.New(apperr.CodeValidation, "No nights in that range fall on the chosen days.")
)
