package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/platform/apperr"
)

// Field error messages, shared so every endpoint words problems the same way.
const (
	msgRequired   = "is required"
	msgBadDate    = "must be a date like 2026-11-14"
	msgBadNumber  = "must be a whole number"
	msgOutOfRange = "is out of range"
	msgBadID      = "must be a valid id"
)

// Params reads query parameters and collects every problem, so the client gets all of
// them in one VALIDATION_FAILED response instead of one at a time.
type Params struct {
	r      *http.Request
	errors map[string]string
}

func NewParams(r *http.Request) *Params { return &Params{r: r, errors: map[string]string{}} }

func (p *Params) fail(name, msg string) {
	if _, seen := p.errors[name]; !seen {
		p.errors[name] = msg
	}
}

// String returns a trimmed value, or fallback when absent.
func (p *Params) String(name, fallback string) string {
	if v := strings.TrimSpace(p.r.URL.Query().Get(name)); v != "" {
		return v
	}
	return fallback
}

// Date reads a YYYY-MM-DD date. Required.
func (p *Params) Date(name string) time.Time {
	v := p.String(name, "")
	if v == "" {
		p.fail(name, msgRequired)
		return time.Time{}
	}
	d, err := time.Parse(time.DateOnly, v)
	if err != nil {
		p.fail(name, msgBadDate)
	}
	return d
}

// Int reads a whole number in [lo, hi], or fallback when absent.
func (p *Params) Int(name string, fallback, lo, hi int) int {
	v := p.String(name, "")
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.fail(name, msgBadNumber)
		return fallback
	}
	if n < lo || n > hi {
		p.fail(name, msgOutOfRange)
	}
	return n
}

// Err returns a VALIDATION_FAILED error if anything was wrong.
func (p *Params) Err() error {
	if len(p.errors) == 0 {
		return nil
	}
	return apperr.Validation(p.errors)
}

// PathID parses a uuid path value.
func PathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, apperr.Validation(map[string]string{name: msgBadID})
	}
	return id, nil
}

// ParseDate parses a YYYY-MM-DD date from a JSON field, recording a problem under name.
func ParseDate(fields map[string]string, name, v string) time.Time {
	d, err := time.Parse(time.DateOnly, strings.TrimSpace(v))
	if err != nil {
		fields[name] = msgBadDate
	}
	return d
}
