package httpx

import (
	"net/http"

	"kaapi/api/internal/platform/apperr"
)

// Header names used across the API.
const (
	HeaderRequestID   = "X-Request-Id"
	HeaderContentType = "Content-Type"
	ContentTypeJSON   = "application/json; charset=utf-8"

	// MaxBodyBytes caps request bodies; enquiries are the largest legitimate payload.
	MaxBodyBytes = 64 << 10
)

type ctxKey int

const requestIDKey ctxKey = iota

// HandlerFunc is an http.HandlerFunc that returns its error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// ErrorBody is the JSON shape of every error response.
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

type ErrorPayload struct {
	Code      apperr.Code    `json:"code"`
	Message   string         `json:"message"`
	Details   apperr.Details `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}
