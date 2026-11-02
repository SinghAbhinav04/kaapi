// Package httpx is the HTTP edge shared by every handler: request IDs, panic recovery,
// JSON in and out, and the one place where any error becomes a response.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/apperr"
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

// RequestID tags every request with an ID (the caller's, if it sent a sane one) and echoes it back.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" || len(id) > 64 || strings.ContainsAny(id, " \r\n") {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// RequestIDFrom returns the request's ID, or "" outside a request.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// Recover is the last line of defence: a panic anywhere below becomes a logged 500 with
// the request ID, instead of a dropped connection.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler { // the client went away; nothing to write
					panic(rec)
				}
				log.ErrorContext(r.Context(), "panic", "request_id", RequestIDFrom(r.Context()),
					"method", r.Method, "path", r.URL.Path, "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
				writeBody(w, r, apperr.New(apperr.CodeInternal, "Something went wrong on our side."))
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// HandlerFunc is an http.HandlerFunc that returns its error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Handle adapts a HandlerFunc: any returned error goes through WriteError.
func Handle(log *slog.Logger, fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(log, w, r, err)
		}
	}
}

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

// WriteError classifies err, logs it at the right level and writes the error envelope.
// Server-side failures log the full cause; client errors log only the code.
func WriteError(log *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	ae := apperr.From(err)
	attrs := []any{"request_id", RequestIDFrom(r.Context()), "method", r.Method, "path", r.URL.Path,
		"code", ae.Code, "status", ae.Status()}
	if ae.Status() >= http.StatusInternalServerError {
		log.ErrorContext(r.Context(), "request failed", append(attrs, "err", err.Error())...)
	} else {
		log.InfoContext(r.Context(), "request rejected", attrs...)
	}
	writeBody(w, r, ae)
}

func writeBody(w http.ResponseWriter, r *http.Request, ae *apperr.Error) {
	WriteJSON(w, ae.Status(), ErrorBody{Error: ErrorPayload{
		Code: ae.Code, Message: ae.Message, Details: ae.Details, RequestID: RequestIDFrom(r.Context()),
	}})
}

// WriteJSON writes v with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(HeaderContentType, ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON reads one JSON object into dst, rejecting unknown fields, trailing data and
// oversized bodies with a client error the caller can return as-is.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		var syntax *json.SyntaxError
		var typ *json.UnmarshalTypeError
		switch {
		case errors.As(err, &tooBig):
			return apperr.New(apperr.CodePayloadTooLarge, "That request is too large.")
		case errors.Is(err, io.EOF):
			return apperr.New(apperr.CodeBadRequest, "The request body is empty.")
		case errors.As(err, &syntax):
			return apperr.New(apperr.CodeBadRequest, "The request body isn't valid JSON.")
		case errors.As(err, &typ):
			return apperr.Validation(map[string]string{typ.Field: "has the wrong type"})
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			return apperr.Validation(map[string]string{field: "is not a known field"})
		default:
			return apperr.Wrap(err, apperr.CodeBadRequest, "The request body couldn't be read.")
		}
	}
	if dec.More() {
		return apperr.New(apperr.CodeBadRequest, "Send a single JSON object.")
	}
	return nil
}

// NotFound and MethodNotAllowed keep the router's own errors in the same envelope.
func NotFound(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteError(log, w, r, apperr.New(apperr.CodeNotFound, "There's nothing at this address."))
	}
}

func MethodNotAllowed(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteError(log, w, r, apperr.New(apperr.CodeMethodNotAllowed, "This address doesn't accept "+r.Method+"."))
	}
}
