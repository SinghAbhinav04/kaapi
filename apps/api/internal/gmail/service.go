// Package gmail connects the resort's mailbox to the inbox: an admin grants access once
// (OAuth, offline), new inbox mail is polled into enquiries, and staff replies go out from
// the same mailbox in the guest's thread. The refresh token is encrypted at rest. No network
// call ever runs inside a database transaction.
package gmail

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/config"
)

// Inbox is where imported mail goes. Has lets the importer skip a message it already
// delivered without downloading it again; Deliver returning an error stops the poll so the
// message is tried again next time (return nil to skip a message for good).
type Inbox interface {
	Has(ctx context.Context, messageID string) (bool, error)
	Deliver(ctx context.Context, in Inbound) (stored bool, err error)
}

type Service struct {
	pool     *pgxpool.Pool
	cfg      config.Gmail
	g        *google
	now      func() time.Time
	log      *slog.Logger
	stateKey []byte
	tokenKey []byte
	inbox    Inbox
	store    store

	mu        sync.Mutex // guards the cached access token; held only while refreshing it
	pollMu    sync.Mutex // one poll or catch-up at a time (the ticker and "Sync mail" can overlap)
	access    string
	accessExp time.Time
}

func NewService(pool *pgxpool.Pool, cfg config.Gmail, secret []byte, now func() time.Time, log *slog.Logger) *Service {
	authURL, oauthBase, apiBase := googleAuthURL, googleOAuthBase, googleAPIBase
	if cfg.OAuthBaseURL != "" {
		authURL, oauthBase = cfg.OAuthBaseURL+"/auth", cfg.OAuthBaseURL
	}
	if cfg.APIBaseURL != "" {
		apiBase = cfg.APIBaseURL
	}
	return &Service{
		pool: pool, cfg: cfg, now: now, log: log,
		g: &google{http: &http.Client{Timeout: httpTimeout}, authURL: authURL, oauthBase: oauthBase, apiBase: apiBase,
			clientID: cfg.ClientID, clientSecret: cfg.ClientSecret, redirectURL: cfg.RedirectURL},
		stateKey: deriveKey(secret, "state"),
		tokenKey: deriveKey(secret, "token"),
	}
}

// SetInbox wires the inbox in; the app does it once at start-up.
func (s *Service) SetInbox(i Inbox) { s.inbox = i }

func (s *Service) Configured() bool { return s.cfg.Configured() }

// ---- connecting

// ConsentURL starts the OAuth round trip for an admin.
func (s *Service) ConsentURL(user uuid.UUID) (string, error) {
	if !s.Configured() {
		return "", ErrNotConfigured
	}
	return s.g.consentURL(signState(s.stateKey, user, s.now().Add(stateTTL))), nil
}

// Callback finishes the round trip: swap the code for tokens, learn the address, and start
// reading from the mailbox's current position (older mail stays where it is).
func (s *Service) Callback(ctx context.Context, code, state string) (string, error) {
	user, err := verifyState(s.stateKey, state, s.now())
	if err != nil {
		return "", err
	}
	tok, err := s.g.exchange(ctx, code)
	if err != nil {
		return "", apperr.Wrap(err, apperr.CodeUnavailable, "Google didn't accept the sign-in. Try connecting again.")
	}
	if tok.RefreshToken == "" {
		return "", apperr.New(apperr.CodeConflict, "Google didn't grant offline access. Remove the app from your Google account's permissions and connect again.")
	}
	p, err := s.g.profile(ctx, tok.AccessToken)
	if err != nil {
		return "", apperr.Wrap(err, apperr.CodeUnavailable, "Connected, but Gmail couldn't be read. Try again.")
	}
	if err := s.save(ctx, p.EmailAddress, tok.RefreshToken, p.HistoryID, &user); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.access, s.accessExp = tok.AccessToken, s.now().Add(time.Duration(tok.ExpiresIn)*time.Second-tokenSkew)
	s.mu.Unlock()
	return p.EmailAddress, nil
}

// Bootstrap connects the mailbox from GMAIL_REFRESH_TOKEN at start-up, so the server is
// connected after every restart, re-seed or fresh deploy without anyone pressing a button.
// A database row that already holds the same grant is left alone (it knows where the last
// poll stopped).
func (s *Service) Bootstrap(ctx context.Context) error {
	refresh := s.cfg.RefreshToken
	if refresh == "" || !s.Configured() {
		return nil
	}
	row, err := s.load(ctx)
	switch {
	case err == nil:
		if cur, err := open(s.tokenKey, row.sealed); err == nil && string(cur) == refresh {
			return nil
		}
	case !errors.Is(err, ErrNotConnected):
		return err
	}
	tok, err := s.g.refresh(ctx, refresh)
	if err != nil {
		return fmt.Errorf("gmail: GMAIL_REFRESH_TOKEN was refused (run cmd/gmail-auth again): %w", err)
	}
	p, err := s.g.profile(ctx, tok.AccessToken)
	if err != nil {
		return err
	}
	history := p.HistoryID
	if row.email == p.EmailAddress && row.historyID != "" {
		history = row.historyID // same mailbox: carry on from the last poll
	}
	if err := s.save(ctx, p.EmailAddress, refresh, history, nil); err != nil {
		return err
	}
	s.mu.Lock()
	s.access, s.accessExp = tok.AccessToken, s.now().Add(time.Duration(tok.ExpiresIn)*time.Second-tokenSkew)
	s.mu.Unlock()
	s.log.Info("gmail connected from environment", "account", p.EmailAddress)
	return nil
}

// RefreshToken returns the stored grant and its mailbox, for cmd/gmail-auth to save into .env.
func (s *Service) RefreshToken(ctx context.Context) (token, email string, err error) {
	row, err := s.load(ctx)
	if err != nil {
		return "", "", err
	}
	plain, err := open(s.tokenKey, row.sealed)
	return string(plain), row.email, err
}

// save encrypts the grant and stores the connection.
func (s *Service) save(ctx context.Context, email, refresh, historyID string, user *uuid.UUID) error {
	sealed, err := seal(s.tokenKey, []byte(refresh))
	if err != nil {
		return err
	}
	return s.store.upsert(ctx, s.pool, email, sealed, historyID, user, s.now())
}

// Disconnect forgets the mailbox and asks Google to revoke the grant (best effort: the
// local token is gone either way).
func (s *Service) Disconnect(ctx context.Context) error {
	row, err := s.load(ctx)
	if errors.Is(err, ErrNotConnected) {
		return nil
	}
	if err != nil {
		return err
	}
	if refresh, err := open(s.tokenKey, row.sealed); err == nil {
		if err := s.g.revoke(ctx, string(refresh)); err != nil {
			s.log.Warn("gmail revoke failed", "err", err)
		}
	}
	s.mu.Lock()
	s.access, s.accessExp = "", time.Time{}
	s.mu.Unlock()
	return s.store.remove(ctx, s.pool)
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	st := Status{Configured: s.Configured()}
	row, err := s.load(ctx)
	if errors.Is(err, ErrNotConnected) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.Connected, st.AccountEmail, st.ConnectedAt, st.LastSyncAt, st.LastError = true, row.email, &row.connectedAt, row.lastSync, row.lastError
	return st, nil
}

// Connected reports whether replies can go out for real. It satisfies the inbox's Mailer.
func (s *Service) Connected(ctx context.Context) bool {
	return s.Configured() && s.store.exists(ctx, s.pool)
}

// ---- sending

// Send delivers a message from the connected mailbox. It satisfies the inbox's Mailer.
func (s *Service) Send(ctx context.Context, m domain.Mail) (domain.SentMail, error) {
	access, from, err := s.token(ctx)
	if err != nil {
		return domain.SentMail{}, err
	}
	raw, err := build(from, m)
	if err != nil {
		return domain.SentMail{}, err
	}
	res, err := s.g.send(ctx, access, raw, m.ThreadID)
	if err != nil {
		return domain.SentMail{}, apperr.Wrap(err, apperr.CodeUnavailable, ErrSendFailed.Message)
	}
	return domain.SentMail{MessageID: res.ID, ThreadID: res.ThreadID}, nil
}

// ---- reading

// Run connects from the environment, polls once straight away, then on every tick until
// ctx ends. A poll with nothing connected costs one query.
func (s *Service) Run(ctx context.Context) {
	if err := s.Bootstrap(ctx); err != nil {
		s.log.Error("gmail bootstrap failed", "err", err)
	}
	poll := func() {
		if n, err := s.Poll(ctx); err != nil && !errors.Is(err, ErrNotConnected) {
			s.log.Error("gmail poll failed", "err", err)
		} else if n > 0 {
			s.log.Info("gmail imported", "count", n)
		}
	}
	poll()
	t := time.NewTicker(s.cfg.PollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			poll()
		}
	}
}

// Poll imports inbox mail added since the last poll and returns how many became enquiries.
// The mailbox position only moves forward once every message up to it has been handed over,
// and the inbox ignores a message it has already seen, so a poll can safely repeat.
func (s *Service) Poll(ctx context.Context) (int, error) {
	if !s.Configured() || s.inbox == nil {
		return 0, nil
	}
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	n, err := s.poll(ctx)
	if err != nil && !errors.Is(err, ErrNotConnected) {
		// Staff read this in the inbox: a known failure's own message, not its cause chain.
		msg := err.Error()
		if ae := (*apperr.Error)(nil); errors.As(err, &ae) {
			msg = ae.Message
		}
		s.store.setError(ctx, s.pool, truncate(msg, maxErrorRunes))
	}
	return n, err
}

func (s *Service) poll(ctx context.Context) (int, error) {
	row, err := s.load(ctx)
	if err != nil {
		return 0, err
	}
	access, account, err := s.tokenFor(ctx, row)
	if err != nil {
		return 0, err
	}
	ids, latest, err := s.g.history(ctx, access, row.historyID)
	if isStatus(err, http.StatusNotFound) { // position too old to replay: list everything since the last sync
		if ids, err = s.g.since(ctx, access, row.resumeFrom()); err == nil {
			var p profile
			if p, err = s.g.profile(ctx, access); err == nil {
				latest = p.HistoryID
			}
		}
	}
	if err != nil {
		return 0, err
	}
	// Every message is handled before the position moves. (An earlier cap of 50 per poll left
	// the position behind, so the next poll replayed the same 50 and never got further.)
	imported, err := s.importIDs(ctx, access, account, uniq(ids))
	if err != nil {
		return imported, err
	}
	return imported, s.store.synced(ctx, s.pool, s.now(), &latest)
}

// CatchUp imports everything that reached the inbox since the last successful sync, however
// long the server was down: the history replay first, then a search for anything it missed.
// Messages already in the inbox are skipped without being downloaded, so it never duplicates.
func (s *Service) CatchUp(ctx context.Context) (int, error) {
	if !s.Configured() || s.inbox == nil {
		return 0, ErrNotConnected
	}
	row, err := s.load(ctx)
	if err != nil {
		return 0, err
	}
	from := row.resumeFrom() // read before the poll moves last_sync_at
	n, err := s.Poll(ctx)
	if err != nil {
		return n, err
	}
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	access, account, err := s.tokenFor(ctx, row)
	if err != nil {
		return n, err
	}
	ids, err := s.g.since(ctx, access, from)
	if err != nil {
		return n, err
	}
	more, err := s.importIDs(ctx, access, account, ids)
	return n + more, err
}

// importIDs downloads and delivers the messages the inbox doesn't have yet.
func (s *Service) importIDs(ctx context.Context, access, account string, ids []string) (int, error) {
	imported := 0
	for _, id := range ids {
		if seen, err := s.inbox.Has(ctx, id); err != nil {
			return imported, err
		} else if seen {
			continue
		}
		m, err := s.g.message(ctx, access, id)
		if isStatus(err, http.StatusNotFound) {
			continue // deleted before we got to it
		}
		if err != nil {
			return imported, err
		}
		if skipLabels(m.LabelIDs) {
			continue
		}
		in := parse(m)
		if in.FromEmail == "" || in.FromEmail == account || in.Text == "" || in.Automated {
			continue // our own sent copies, bounces and auto-replies, empty mail
		}
		stored, err := s.inbox.Deliver(ctx, in)
		if err != nil {
			return imported, fmt.Errorf("gmail: deliver %s: %w", id, err)
		}
		if stored {
			imported++
		}
	}
	return imported, nil
}

func skipLabels(labels []string) bool {
	return slices.ContainsFunc(labels, func(l string) bool { return l == "SENT" || l == "DRAFT" || l == "SPAM" || l == "TRASH" })
}

func uniq(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ---- tokens and storage

type stored struct {
	email       string
	sealed      []byte
	historyID   string
	connectedAt time.Time
	lastSync    *time.Time
	lastError   *string
}

// resumeFrom is where a catch-up starts: just before the last sync, or the connection if
// there hasn't been one.
func (r stored) resumeFrom() time.Time {
	t := r.connectedAt
	if r.lastSync != nil {
		t = *r.lastSync
	}
	return t.Add(-catchUpMargin)
}

func (s *Service) load(ctx context.Context) (stored, error) { return s.store.load(ctx, s.pool) }

// token returns a live access token and the mailbox address.
func (s *Service) token(ctx context.Context) (string, string, error) {
	row, err := s.load(ctx)
	if err != nil {
		return "", "", err
	}
	return s.tokenFor(ctx, row)
}

// tokenFor is tokenLocked under the lock, for callers that already loaded the row.
func (s *Service) tokenFor(ctx context.Context, row stored) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenLocked(ctx, row)
}

// tokenLocked refreshes the access token when it's close to expiry. Callers hold s.mu.
func (s *Service) tokenLocked(ctx context.Context, row stored) (string, string, error) {
	if s.access != "" && s.now().Before(s.accessExp) {
		return s.access, row.email, nil
	}
	refresh, err := open(s.tokenKey, row.sealed)
	if err != nil {
		return "", "", fmt.Errorf("gmail: stored token unreadable (was JWE_SECRET changed?): %w", err)
	}
	tok, err := s.g.refresh(ctx, string(refresh))
	if isStatus(err, http.StatusBadRequest) || isStatus(err, http.StatusUnauthorized) {
		// In Google's Testing mode a refresh token lasts 7 days; a revoked one ends the same way.
		return "", "", apperr.Wrap(err, apperr.CodeConflict,
			"Google no longer accepts the mailbox connection, so no email is going out or coming in. An administrator needs to reconnect Gmail (go run ./cmd/gmail-auth).")
	}
	if err != nil {
		return "", "", err
	}
	s.access, s.accessExp = tok.AccessToken, s.now().Add(time.Duration(tok.ExpiresIn)*time.Second-tokenSkew)
	return s.access, row.email, nil
}
