// Package outbox records every message the system sends. Until a mailbox is connected,
// email and WhatsApp are simulated, so staff (and reviewers) read them here; once Gmail is
// connected, email goes out for real and the copy here says so.
package outbox

import (
	"context"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/emailtpl"
)

type Service struct {
	pool   *pgxpool.Pool
	store  store
	mailer Mailer // nil: messages are simulated
	links  emailtpl.Links
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// SetMailer connects real delivery; the app does it once at start-up.
func (s *Service) SetMailer(m Mailer) { s.mailer = m }

// SetLinks gives emails' buttons the site's address.
func (s *Service) SetLinks(l emailtpl.Links) { s.links = l }

// List returns the newest messages first.
func (s *Service) List(ctx context.Context, limit int) ([]Message, error) {
	return s.store.list(ctx, s.pool, limit)
}

// Send delivers a staff-written email. With a mailbox connected it goes out for real (the
// network call happens before anything is written, outside any transaction); otherwise it
// is recorded as a simulation.
func (s *Service) Send(ctx context.Context, actor uuid.UUID, c Compose) (Message, error) {
	c, err := validate(c)
	if err != nil {
		return Message{}, err
	}
	name, team, err := s.store.signer(ctx, s.pool, actor)
	if err != nil {
		return Message{}, err
	}
	// Signed by the sender, like a reply; the stored copy is exactly what was sent.
	e := emailtpl.Reply(c.Subject, c.Body, emailtpl.SignOff(name, team), s.links)
	c.Body = e.Text
	status, sent := domain.OutboxSimulated, domain.SentMail{}
	if s.mailer != nil && s.mailer.Connected(ctx) {
		if sent, err = s.mailer.Send(ctx, domain.Mail{To: c.To, Subject: c.Subject, Body: e.Text, HTML: e.HTML}); err != nil {
			return Message{}, err
		}
		status = domain.OutboxSent
	}
	var external *string
	if sent.MessageID != "" {
		external = &sent.MessageID
	}
	// The email has gone; record it even if the request was cancelled meanwhile.
	return s.store.insertStaffMessage(context.WithoutCancel(ctx), s.pool, actor, c, status, external)
}

// Deliver sends queued guest emails through the connected mailbox: claim a batch, send each
// with no transaction open, then record the outcome. With no mailbox connected they're marked
// SIMULATED, as before. Safe to run in several processes at once.
func (s *Service) Deliver(ctx context.Context, limit int) (DeliverResult, error) {
	var r DeliverResult
	batch, err := s.store.claim(ctx, s.pool, limit)
	if err != nil || len(batch) == 0 {
		return r, err
	}
	connected := s.mailer != nil && s.mailer.Connected(ctx)
	for _, m := range batch {
		// Recording uses its own context: a mail that went out must be marked sent even if
		// the worker is shutting down.
		rec := context.WithoutCancel(ctx)
		// Demo and test addresses can never receive mail; sending them would only bounce back
		// into the inbox (and be polled in as enquiries).
		if !connected || Reserved(m.Recipient) {
			r.Simulated++
			if err := s.store.markSent(rec, s.pool, m.ID, domain.OutboxSimulated, nil); err != nil {
				return r, err
			}
			continue
		}
		e := emailtpl.Render(m.Kind, m.Subject, m.Body, s.links)
		sent, sendErr := s.mailer.Send(ctx, domain.Mail{To: m.Recipient, Subject: m.Subject, Body: m.Body, HTML: e.HTML})
		if sendErr == nil {
			r.Sent++
			id := sent.MessageID
			if err := s.store.markSent(rec, s.pool, m.ID, domain.OutboxSent, &id); err != nil {
				return r, err
			}
			continue
		}
		final := m.Attempts >= maxAttempts
		if final {
			r.Failed++
		} else {
			r.Retrying++
		}
		wait := firstBackoff << (m.Attempts - 1) // 30 s, 1 min, 2 min, 4 min
		if err := s.store.markFailed(rec, s.pool, m.ID, final, wait, truncate(sendErr.Error(), maxErrorLen)); err != nil {
			return r, err
		}
	}
	return r, nil
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func validate(c Compose) (Compose, error) {
	c.To = strings.ToLower(strings.TrimSpace(c.To))
	c.Subject = strings.Join(strings.Fields(c.Subject), " ")
	c.Body = strings.TrimSpace(c.Body)
	fields := map[string]string{}
	if a, err := mail.ParseAddress(c.To); err != nil || a.Address != c.To {
		fields["to"] = "must be one email address"
	}
	if c.Subject == "" || len([]rune(c.Subject)) > maxSubjectLen {
		fields["subject"] = "must be 1 to 200 characters"
	}
	if c.Body == "" || len([]rune(c.Body)) > maxBodyLen {
		fields["body"] = "must be 1 to 4000 characters"
	}
	if len(fields) > 0 {
		return c, apperr.Validation(fields)
	}
	return c, nil
}
