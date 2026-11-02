package ai

import (
	"errors"
	"fmt"
	"time"
)

// ErrUnavailable means "try again later": every model was rate-limited, down or unusable.
// The classifier then uses the keyword rules and marks the enquiry FALLBACK.
var ErrUnavailable = errors.New("ai: provider unavailable")

// FailKind says what went wrong with one model, which decides what the chain does next.
type FailKind string

const (
	FailRateLimited FailKind = "RATE_LIMITED" // 429: rest this model until its quota refills
	FailUnavailable FailKind = "UNAVAILABLE"  // 5xx, timeout, network: rest it briefly
	FailRejected    FailKind = "REJECTED"     // 4xx: bad key, model gone or unsupported; rest it for long
	FailBadOutput   FailKind = "BAD_OUTPUT"   // answered, but not in the agreed shape; try the next model
)

// How long a model rests after each kind of failure, unless the provider says otherwise.
const (
	restRateLimited = time.Minute
	restUnavailable = 30 * time.Second
	restRejected    = time.Hour
	maxRest         = 6 * time.Hour
)

// ProviderError is one model's failure.
type ProviderError struct {
	Model      string
	Kind       FailKind
	RetryAfter time.Duration // from the provider, when it says
	Err        error
}

func (e *ProviderError) Error() string { return fmt.Sprintf("%s: %s: %v", e.Model, e.Kind, e.Err) }
func (e *ProviderError) Unwrap() error { return e.Err }

// Is lets callers treat temporary failures as ErrUnavailable.
func (e *ProviderError) Is(target error) bool {
	return target == ErrUnavailable && (e.Kind == FailRateLimited || e.Kind == FailUnavailable)
}

// rest is how long the chain should leave this model alone.
func (e *ProviderError) rest() time.Duration {
	switch e.Kind {
	case FailRateLimited:
		if e.RetryAfter > 0 {
			return min(e.RetryAfter, maxRest)
		}
		return restRateLimited
	case FailUnavailable:
		return restUnavailable
	case FailRejected:
		return restRejected
	default:
		return 0
	}
}

func failKindForStatus(status int) FailKind {
	switch {
	case status == 429:
		return FailRateLimited
	case status >= 500 || status == 408:
		return FailUnavailable
	default:
		return FailRejected
	}
}
