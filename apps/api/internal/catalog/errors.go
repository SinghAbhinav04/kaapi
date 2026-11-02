package catalog

import "kaapi/api/internal/platform/apperr"

var (
	ErrRoomNotFound = apperr.New(apperr.CodeNotFound, "We couldn't find that room.")
	ErrPlanNotFound = apperr.New(apperr.CodeNotFound, "We couldn't find that rate.")
	ErrBadRange     = apperr.New(apperr.CodeValidation, "Choose a date range of up to six weeks.")
)
