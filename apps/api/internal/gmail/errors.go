package gmail

import "kaapi/api/internal/platform/apperr"

var (
	ErrNotConfigured = apperr.New(apperr.CodeConflict, "Gmail isn't set up on the server yet. Add GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET.")
	ErrNotConnected  = apperr.New(apperr.CodeConflict, "No Gmail mailbox is connected.")
	ErrBadState      = apperr.New(apperr.CodeBadRequest, "That Gmail sign-in link has expired. Start again from the inbox.")
	ErrSendFailed    = apperr.New(apperr.CodeUnavailable, "Gmail didn't accept the message. Nothing was sent; try again in a moment.")
)
