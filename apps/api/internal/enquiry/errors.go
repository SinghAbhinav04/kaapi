package enquiry

import "kaapi/api/internal/platform/apperr"

var (
	ErrNotFound      = apperr.New(apperr.CodeNotFound, "We couldn't find that enquiry.")
	ErrNoDraft       = apperr.New(apperr.CodeConflict, "There's no draft to send yet.")
	ErrAlreadySent   = apperr.New(apperr.CodeConflict, "This draft has already been sent.")
	ErrSending       = apperr.New(apperr.CodeConflict, "This reply is being sent right now.")
	ErrNoRecipient   = apperr.New(apperr.CodeConflict, "This enquiry has no email or phone to reply to.")
	ErrTooMany       = apperr.New(apperr.CodeRateLimited, "We've received several messages from you in the last hour. We'll reply soon.")
	ErrEmptyMessage  = apperr.Validation(map[string]string{"message": "is required"})
	ErrNeedsContact  = apperr.Validation(map[string]string{"email": "add an email or a phone number so we can reply"})
	ErrBadLane       = apperr.Validation(map[string]string{"lane": "is not a lane"})
	ErrBadChannel    = apperr.Validation(map[string]string{"channel": "is not a channel"})
	ErrBadCategory   = apperr.Validation(map[string]string{"category": "is not a category"})
	ErrNothingToSave = apperr.New(apperr.CodeValidation, "Nothing to change.")
)
