package auth

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
)

// Claims are what an access token carries: who, in which role, on which session.
// Deliberately no name, email or phone: tokens shouldn't hold personal data.
type Claims struct {
	UserID    uuid.UUID   `json:"sub"`
	Role      domain.Role `json:"role"`
	SessionID uuid.UUID   `json:"sid"`
	IssuedAt  int64       `json:"iat"`
	ExpiresAt int64       `json:"exp"`
}

// Principal is the caller of a request, as established by the gateway.
type Principal struct {
	UserID    uuid.UUID
	Role      domain.Role
	SessionID uuid.UUID
}

func (p Principal) IsAuthenticated() bool { return p.UserID != uuid.Nil }

// User is the public view of an account.
type User struct {
	ID    uuid.UUID
	Email string
	Name  string
	Role  domain.Role
}

// Tokens is a freshly issued pair.
type Tokens struct {
	Access           string
	AccessExpiresAt  time.Time
	Refresh          string
	RefreshExpiresAt time.Time
}

// ClientInfo is recorded on a session so a user can recognise it.
type ClientInfo struct {
	UserAgent string
	IP        string
}

// Lifetimes and limits. Access tokens are short so a stolen one is soon useless;
// refresh tokens rotate on every use.
const (
	AccessTTL         = 15 * time.Minute
	RefreshTTL        = 30 * 24 * time.Hour
	OTPTTL            = 5 * time.Minute
	OTPLength         = 6
	OTPMaxAttempts    = 5
	OTPMaxPerWindow   = 3
	OTPWindow         = 10 * time.Minute
	refreshTokenBytes = 32
	userAgentMaxLen   = 300

	// SessionCheckTTL is how long a "this session is live" answer is reused. Revoking in this
	// process (logout, refresh reuse) takes effect at once; in other processes within this.
	SessionCheckTTL = 30 * time.Second
	// MaxTokenBytes: anything longer isn't one of ours, so it's refused before decrypting.
	MaxTokenBytes = 4096
	// clockSkew tolerates a token issued by a server whose clock runs slightly ahead.
	clockSkew = time.Minute
	// maxCachedSessions bounds the live-session cache's memory.
	maxCachedSessions = 50_000
)
