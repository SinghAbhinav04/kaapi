package auth

import "kaapi/api/internal/platform/apperr"

var (
	ErrUnauthenticated = apperr.New(apperr.CodeUnauthenticated, "Please sign in.")
	ErrTokenInvalid    = apperr.New(apperr.CodeUnauthenticated, "Your session has ended. Please sign in again.")
	ErrBadCredentials  = apperr.New(apperr.CodeUnauthenticated, "That email and password don't match.")
	ErrOTPInvalid      = apperr.New(apperr.CodeUnauthenticated, "That code isn't right, or it has expired.")
	ErrOTPRateLimited  = apperr.New(apperr.CodeRateLimited, "Too many codes requested. Please wait a few minutes.")
	ErrForbidden       = apperr.New(apperr.CodeForbidden, "You don't have access to this.")
	ErrBadOrigin       = apperr.New(apperr.CodeForbidden, "This request came from a site we don't recognise.")
)
