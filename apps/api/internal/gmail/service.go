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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/config"
)

// Deliver hands one inbound email to the inbox. Returning an error stops the poll so the
// message is tried again next time; return nil to skip a message for good.
type Deliver func(ctx context.Context, in Inbound) error

type Service struct {
	pool     *pgxpool.Pool
	cfg      config.Gmail
	g        *google
	now      func() time.Time
	log      *slog.Logger
	stateKey []byte
	tokenKey []byte
	deliver  Deliver

	mu        sync.Mutex // guards the cached access token and serialises polls
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

// SetDeliver wires the inbox in; the API does it once at start-up.
func (s *Service) SetDeliver(d Deliver) { s.deliver = d }

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
	sealed, err := seal(s.tokenKey, []byte(tok.RefreshToken))
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO integrations (provider, account_email, refresh_token_enc, history_id, connected_by, connected_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider) DO UPDATE SET account_email = $2, refresh_token_enc = $3, history_id = $4, connected_by = $5,
			connected_at = $6, last_sync_at = NULL, last_error = NULL`,
		provider, p.EmailAddress, sealed, p.HistoryID, user, s.now()); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.access, s.accessExp = tok.AccessToken, s.now().Add(time.Duration(tok.ExpiresIn)*time.Second-tokenSkew)
	s.mu.Unlock()
	return p.EmailAddress, nil
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
	_, err = s.pool.Exec(ctx, `DELETE FROM integrations WHERE provider = $1`, provider)
	return err
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
	var ok bool
	_ = s.pool.QueryRow(ctx, `SELECT true FROM integrations WHERE provider = $1`, provider).Scan(&ok)
	return ok && s.Configured()
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

// Run polls the mailbox until ctx ends. A poll with nothing connected costs one query.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(s.cfg.PollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.Poll(ctx); err != nil && !errors.Is(err, ErrNotConnected) {
				s.log.Error("gmail poll failed", "err", err)
			} else if n > 0 {
				s.log.Info("gmail imported", "count", n)
			}
		}
	}
}

// Poll imports inbox mail added since the last poll and returns how many became enquiries.
// The mailbox position only moves forward once every message up to it has been handed over,
// and the inbox ignores a message it has already seen, so a poll can safely repeat.
func (s *Service) Poll(ctx context.Context) (int, error) {
	if !s.Configured() || s.deliver == nil {
		return 0, nil
	}
	s.mu.Lock() // one poll at a time: the ticker and a "sync now" press can overlap
	defer s.mu.Unlock()
	n, err := s.poll(ctx)
	if err != nil && !errors.Is(err, ErrNotConnected) {
		_, _ = s.pool.Exec(ctx, `UPDATE integrations SET last_error = $2 WHERE provider = $1`, provider, truncate(err.Error(), maxErrorRunes))
	}
	return n, err
}

func (s *Service) poll(ctx context.Context) (int, error) {
	row, err := s.load(ctx)
	if err != nil {
		return 0, err
	}
	access, account, err := s.tokenLocked(ctx, row)
	if err != nil {
		return 0, err
	}
	ids, latest, err := s.g.history(ctx, access, row.historyID)
	if isStatus(err, http.StatusNotFound) { // position too old to replay: take the last day instead
		if ids, err = s.g.recent(ctx, access); err == nil {
			var p profile
			if p, err = s.g.profile(ctx, access); err == nil {
				latest = p.HistoryID
			}
		}
	}
	if err != nil {
		return 0, err
	}
	ids = uniq(ids)
	advance := true
	if len(ids) > maxPerPoll { // the rest come next poll; don't move past them
		ids, advance = ids[:maxPerPoll], false
	}
	imported := 0
	for _, id := range ids {
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
		if in.FromEmail == "" || in.FromEmail == account || in.Text == "" {
			continue // our own sent copies, bounces with no sender, empty mail
		}
		if err := s.deliver(ctx, in); err != nil {
			return imported, fmt.Errorf("gmail: deliver %s: %w", id, err)
		}
		imported++
	}
	q := `UPDATE integrations SET last_sync_at = $2, last_error = NULL WHERE provider = $1`
	args := []any{provider, s.now()}
	if advance {
		q = `UPDATE integrations SET last_sync_at = $2, last_error = NULL, history_id = $3 WHERE provider = $1`
		args = append(args, latest)
	}
	_, err = s.pool.Exec(ctx, q, args...)
	return imported, err
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

func (s *Service) load(ctx context.Context) (stored, error) {
	var r stored
	var history *string
	err := s.pool.QueryRow(ctx, `SELECT account_email, refresh_token_enc, history_id, connected_at, last_sync_at, last_error
		FROM integrations WHERE provider = $1`, provider).Scan(&r.email, &r.sealed, &history, &r.connectedAt, &r.lastSync, &r.lastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotConnected
	}
	if history != nil {
		r.historyID = *history
	}
	return r, err
}

// token returns a live access token and the mailbox address.
func (s *Service) token(ctx context.Context) (string, string, error) {
	row, err := s.load(ctx)
	if err != nil {
		return "", "", err
	}
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
		return "", "", apperr.Wrap(err, apperr.CodeConflict, "Google no longer accepts the connection. Reconnect Gmail from the inbox.")
	}
	if err != nil {
		return "", "", err
	}
	s.access, s.accessExp = tok.AccessToken, s.now().Add(time.Duration(tok.ExpiresIn)*time.Second-tokenSkew)
	return s.access, row.email, nil
}
