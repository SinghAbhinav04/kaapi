package apperr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestEveryCodeHasAStatus(t *testing.T) {
	for _, c := range Codes {
		if s := c.HTTPStatus(); s < 400 || s > 599 {
			t.Errorf("%s maps to %d", c, s)
		}
	}
	if Code("SOMETHING_NEW").HTTPStatus() != http.StatusInternalServerError {
		t.Error("unknown codes must be treated as 500")
	}
}

type domainErr struct{}

func (domainErr) Error() string { return "domain" }
func (domainErr) AppError() *Error {
	return New(CodeSoldOut, "No rooms left.").With(Details{"nights": []string{"2026-12-30"}})
}

func TestFrom(t *testing.T) {
	sentinel := New(CodeHoldNotFound, "That hold doesn't exist.")
	cases := []struct {
		name string
		err  error
		code Code
	}{
		{"coder wins, even wrapped", fmt.Errorf("booking: %w", domainErr{}), CodeSoldOut},
		{"wrapped sentinel", fmt.Errorf("x: %w", sentinel), CodeHoldNotFound},
		{"cancelled", context.Canceled, CodeClientClosed},
		{"deadline", fmt.Errorf("q: %w", context.DeadlineExceeded), CodeTimeout},
		{"unique violation", &pgconn.PgError{Code: "23505"}, CodeConflict},
		{"check violation", &pgconn.PgError{Code: "23514", ConstraintName: "night_inventory_no_oversell"}, CodeConflict},
		{"deadlock after retries", &pgconn.PgError{Code: "40P01"}, CodeUnavailable},
		{"anything else", errors.New("disk on fire"), CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := From(tc.err); got.Code != tc.code {
				t.Fatalf("got %s, want %s", got.Code, tc.code)
			}
		})
	}
}

func TestInternalNeverLeaksTheCause(t *testing.T) {
	e := From(errors.New("password=hunter2 connection refused"))
	if e.Message != "Something went wrong on our side." {
		t.Fatalf("message leaks internals: %q", e.Message)
	}
	if !errors.Is(e, e.cause) {
		t.Fatal("cause must still be available for logging")
	}
}

func TestWithDoesNotMutateSentinel(t *testing.T) {
	base := New(CodeSoldOut, "No rooms left.")
	_ = base.With(Details{"nights": 1})
	if base.Details != nil {
		t.Fatal("sentinel was mutated")
	}
}
