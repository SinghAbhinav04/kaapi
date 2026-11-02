package apperr

import "net/http"

// Code is a stable, machine-readable error identifier. Clients switch on it; the message is for people.
type Code string

const (
	CodeBadRequest          Code = "BAD_REQUEST"
	CodeValidation          Code = "VALIDATION_FAILED"
	CodePayloadTooLarge     Code = "PAYLOAD_TOO_LARGE"
	CodeUnauthenticated     Code = "UNAUTHENTICATED"
	CodeForbidden           Code = "FORBIDDEN"
	CodeNotFound            Code = "NOT_FOUND"
	CodeMethodNotAllowed    Code = "METHOD_NOT_ALLOWED"
	CodeConflict            Code = "CONFLICT"
	CodeRateLimited         Code = "RATE_LIMITED"
	CodeIdempotencyMismatch Code = "IDEMPOTENCY_MISMATCH"

	// booking domain
	CodeInvalidStay     Code = "INVALID_STAY"
	CodeNotOnSale       Code = "NOT_ON_SALE"
	CodeSoldOut         Code = "SOLD_OUT"
	CodeStayRestricted  Code = "STAY_RESTRICTED"
	CodePromoInvalid    Code = "PROMO_INVALID"
	CodeTooManyGuests   Code = "TOO_MANY_GUESTS"
	CodeHoldNotFound    Code = "HOLD_NOT_FOUND"
	CodeHoldExpired     Code = "HOLD_EXPIRED"
	CodeAllotmentTooLow Code = "ALLOTMENT_TOO_LOW"

	CodeClientClosed Code = "CLIENT_CLOSED_REQUEST"
	CodeTimeout      Code = "TIMEOUT"
	CodeUnavailable  Code = "SERVICE_UNAVAILABLE"
	CodeInternal     Code = "INTERNAL"
)

// statusClientClosed is nginx's convention for a request the client gave up on.
const statusClientClosed = 499

var httpStatus = map[Code]int{
	CodeBadRequest:          http.StatusBadRequest,
	CodeValidation:          http.StatusUnprocessableEntity,
	CodePayloadTooLarge:     http.StatusRequestEntityTooLarge,
	CodeUnauthenticated:     http.StatusUnauthorized,
	CodeForbidden:           http.StatusForbidden,
	CodeNotFound:            http.StatusNotFound,
	CodeMethodNotAllowed:    http.StatusMethodNotAllowed,
	CodeConflict:            http.StatusConflict,
	CodeRateLimited:         http.StatusTooManyRequests,
	CodeIdempotencyMismatch: http.StatusConflict,
	CodeInvalidStay:         http.StatusUnprocessableEntity,
	CodeNotOnSale:           http.StatusUnprocessableEntity,
	CodeSoldOut:             http.StatusConflict,
	CodeStayRestricted:      http.StatusUnprocessableEntity,
	CodePromoInvalid:        http.StatusUnprocessableEntity,
	CodeTooManyGuests:       http.StatusUnprocessableEntity,
	CodeHoldNotFound:        http.StatusNotFound,
	CodeHoldExpired:         http.StatusGone,
	CodeAllotmentTooLow:     http.StatusConflict,
	CodeClientClosed:        statusClientClosed,
	CodeTimeout:             http.StatusGatewayTimeout,
	CodeUnavailable:         http.StatusServiceUnavailable,
	CodeInternal:            http.StatusInternalServerError,
}

// Codes lists every code; a test checks each one has a status.
var Codes = func() []Code {
	out := make([]Code, 0, len(httpStatus))
	for c := range httpStatus {
		out = append(out, c)
	}
	return out
}()

// HTTPStatus returns the status for a code; unknown codes are treated as server errors.
func (c Code) HTTPStatus() int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}
