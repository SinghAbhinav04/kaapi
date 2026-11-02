// Package apperr is the single error model for the API: a closed set of error codes, one
// table mapping each code to an HTTP status, and an Error type with a safe message and
// structured details. Domain packages return their own errors; anything that reports a
// code (Coder) or wraps an *Error is turned into a response by httpx without per-handler
// switches.
package apperr

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Details carries structured context for clients, e.g. {"nights": ["2026-12-30"]}.
type Details map[string]any

// Error is a classified error. Message is safe to show to users; the wrapped cause is only logged.
type Error struct {
	Code    Code
	Message string
	Details Details
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.cause.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// Is matches by code, so errors.Is(err, SomeSentinel) holds for any error of that kind,
// including ones built with a more specific message.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Status is the HTTP status for this error.
func (e *Error) Status() int { return e.Code.HTTPStatus() }

func New(code Code, message string) *Error { return &Error{Code: code, Message: message} }

func Wrap(cause error, code Code, message string) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// With returns a copy with details attached, leaving shared sentinel errors untouched.
func (e *Error) With(d Details) *Error {
	c := *e
	c.Details = d
	return &c
}

// Validation builds a VALIDATION_FAILED error with one message per field.
func Validation(fields map[string]string) *Error {
	return &Error{Code: CodeValidation, Message: "Some fields need attention.", Details: Details{"fields": fields}}
}

// Coder is implemented by domain errors that know their own code and details.
type Coder interface{ AppError() *Error }

// From classifies any error. The order matters: rich domain errors first, then plain
// *Error values, then well-known infrastructure errors, and finally a generic INTERNAL
// that never exposes the underlying message.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var coder Coder
	if errors.As(err, &coder) {
		return coder.AppError()
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	switch {
	case errors.Is(err, context.Canceled):
		return Wrap(err, CodeClientClosed, "The request was cancelled.")
	case errors.Is(err, context.DeadlineExceeded):
		return Wrap(err, CodeTimeout, "This took too long. Please try again.")
	case errors.Is(err, pgx.ErrNoRows):
		return Wrap(err, CodeNotFound, "We couldn't find that.")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return Wrap(err, CodeConflict, "That already exists.")
		case "23514", "23503": // check_violation, foreign_key_violation: a rule the app should have caught first
			return Wrap(err, CodeConflict, "That change isn't allowed.")
		case "40001", "40P01": // serialization_failure, deadlock_detected (after retries)
			return Wrap(err, CodeUnavailable, "We're busy right now. Please try again.")
		case "57014": // query_canceled
			return Wrap(err, CodeTimeout, "This took too long. Please try again.")
		}
	}
	return Wrap(err, CodeInternal, "Something went wrong on our side.")
}
