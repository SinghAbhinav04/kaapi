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

// welcomeBody greets a guest whose first sign-in created their account.
const welcomeBody = "Welcome to Kaapi Estate.\n\n" +
	"You've signed in for the first time, so we've made you a guest account with this email. " +
	"From My bookings you can see your stays, and you'll always sign in with a code we email you: there's no password to remember.\n\n" +
	"We only keep what we need for your bookings, and we never ask for passport details online.\n\n" +
	"If this wasn't you, you can ignore this email; nobody can sign in without the codes we send here."

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

// StaffChallenge is the result of a correct staff password: a code was emailed to SentTo
// (masked for display), or, for a password-only account, SignedIn is set and the session
// is already open.
type StaffChallenge struct {
	SentTo   string
	SignedIn bool
	User     User
	Tokens   Tokens
}
