package auth

import "kaapi/api/internal/platform/apperr"

var (
	ErrUnauthenticated = apperr.New(apperr.CodeUnauthenticated, "Please sign in.")
	ErrTokenInvalid    = apperr.New(apperr.CodeUnauthenticated, "Your session has ended. Please sign in again.")
	ErrBadCredentials  = apperr.New(apperr.CodeUnauthenticated, "That email and password don't match.")
	ErrOTPInvalid      = apperr.New(apperr.CodeUnauthenticated, "That code isn't right, or it has expired.")
	// Shown only after a correct code, so it tells nothing to someone who doesn't own the address.
	ErrOTPStaffEmail  = apperr.New(apperr.CodeForbidden, "This email belongs to a staff account. Sign in with your staff password to book with it; you'll come straight back to this booking.")
	ErrOTPRateLimited = apperr.New(apperr.CodeRateLimited, "Too many codes requested. Please wait a few minutes.")
	ErrForbidden      = apperr.New(apperr.CodeForbidden, "You don't have access to this.")
	ErrBadOrigin      = apperr.New(apperr.CodeForbidden, "This request came from a site we don't recognise.")
)
