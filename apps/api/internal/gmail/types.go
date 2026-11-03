package gmail

import "time"

const (
	provider = "GMAIL"

	googleAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	googleOAuthBase = "https://oauth2.googleapis.com" // token and revoke
	googleAPIBase   = "https://gmail.googleapis.com"

	// Read the inbox and send as the mailbox; nothing that can delete or change mail.
	scopes = "https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.send"

	stateTTL     = 10 * time.Minute // how long the consent screen may take
	httpTimeout  = 20 * time.Second
	tokenSkew    = time.Minute // refresh the access token this long before Google says it expires
	maxPerPoll   = 50          // messages imported per poll; the rest wait for the next
	maxCatchUp   = 2000        // a catch-up's ceiling, so one sync can't run unbounded
	listPageSize = 100
	// catchUpMargin re-reads a little before the last sync: Gmail's "after:" is coarse, and
	// anything already imported is skipped without downloading it.
	catchUpMargin = time.Hour
	maxBodyRunes  = 5000 // the enquiries table's limit
	maxErrorRunes = 300
)

// Status is what the console shows about the mailbox.
type Status struct {
	Configured   bool // the server has an OAuth client
	Connected    bool
	AccountEmail string
	ConnectedAt  *time.Time
	LastSyncAt   *time.Time
	LastError    *string
}

// Inbound is one email turned into what the inbox needs.
type Inbound struct {
	MessageID    string // Gmail's id
	ThreadID     string
	RFCMessageID string // the Message-ID header
	FromName     string
	FromEmail    string
	Subject      string
	Text         string
}
