package apperr_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"kaapi/api/internal/platform/apperr"
)

func TestEveryCodeHasAStatus(t *testing.T) {
	for _, c := range apperr.Codes {
		if s := c.HTTPStatus(); s < 400 || s > 599 {
			t.Errorf("%s maps to %d", c, s)
		}
	}
	if apperr.Code("SOMETHING_NEW").HTTPStatus() != http.StatusInternalServerError {
		t.Error("unknown codes must be treated as 500")
	}
}

type domainErr struct{}

func (domainErr) Error() string { return "domain" }
func (domainErr) AppError() *apperr.Error {
	return apperr.New(apperr.CodeSoldOut, "No rooms left.").With(apperr.Details{"nights": []string{"2026-12-30"}})
}

func TestFrom(t *testing.T) {
	sentinel := apperr.New(apperr.CodeHoldNotFound, "That hold doesn't exist.")
	cases := []struct {
		name string
		err  error
		code apperr.Code
	}{
		{"coder wins, even wrapped", fmt.Errorf("booking: %w", domainErr{}), apperr.CodeSoldOut},
		{"wrapped sentinel", fmt.Errorf("x: %w", sentinel), apperr.CodeHoldNotFound},
		{"cancelled", context.Canceled, apperr.CodeClientClosed},
		{"deadline", fmt.Errorf("q: %w", context.DeadlineExceeded), apperr.CodeTimeout},
		{"unique violation", &pgconn.PgError{Code: "23505"}, apperr.CodeConflict},
		{"check violation", &pgconn.PgError{Code: "23514", ConstraintName: "night_inventory_no_oversell"}, apperr.CodeConflict},
		{"deadlock after retries", &pgconn.PgError{Code: "40P01"}, apperr.CodeUnavailable},
		{"anything else", errors.New("disk on fire"), apperr.CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apperr.From(tc.err); got.Code != tc.code {
				t.Fatalf("got %s, want %s", got.Code, tc.code)
			}
		})
	}
}

func TestInternalNeverLeaksTheCause(t *testing.T) {
	e := apperr.From(errors.New("password=hunter2 connection refused"))
	if e.Message != "Something went wrong on our side." {
		t.Fatalf("message leaks internals: %q", e.Message)
	}
	if errors.Unwrap(e) == nil {
		t.Fatal("cause must still be available for logging")
	}
}

func TestWithDoesNotMutateSentinel(t *testing.T) {
	base := apperr.New(apperr.CodeSoldOut, "No rooms left.")
	_ = base.With(apperr.Details{"nights": 1})
	if base.Details != nil {
		t.Fatal("sentinel was mutated")
	}
}
