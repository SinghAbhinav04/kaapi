// Package campaign sends offers and seasonal emails to guests who asked for them.
//
// It is an observer: the offers service (package admin) announces "offer published" to its
// watchers, and this package, registered as one, turns the announcement into a campaign in
// the same transaction. Staff can also write festive or announcement campaigns directly. The
// worker fans each due campaign out into one stored email per opted-in guest (Dispatch), and
// the outbox sends them at its own pace. Every email carries an unsubscribe link.
package campaign

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
	now   func() time.Time
	site  string // for unsubscribe links
}

func NewService(pool *pgxpool.Pool, now func() time.Time, site string) *Service {
	return &Service{pool: pool, now: now, site: strings.TrimRight(site, "/")}
}

// OfferPublished is the watcher: an offer staff chose to announce becomes a campaign, written
// in the offer's own transaction so one never exists without the other. Once per offer.
func (s *Service) OfferPublished(ctx context.Context, tx pgx.Tx, o Offer, by uuid.UUID) error {
	subject := "A new offer at Kaapi Estate: " + o.Code
	how := fmt.Sprintf("Use code %s when you book direct on our website.", o.Code)
	if o.AutoApply {
		how = "It applies by itself when you book direct on our website: no code needed."
	}
	body := fmt.Sprintf("%s\n\n%s\n\nStays from %s to %s, book by %s%s.",
		o.Description, how, o.StayFrom.Format("2 January"), o.StayTo.Format("2 January 2006"), o.BookTo.Format("2 January 2006"),
		minNights(o.MinNights))
	return s.store.insertOfferCampaign(ctx, tx, subject, body, o.ID, s.now(), by)
}

func minNights(n int) string {
	if n > 1 {
		return fmt.Sprintf(", for stays of %d nights or more", n)
	}
	return ""
}

// Create schedules a festive or announcement campaign.
func (s *Service) Create(ctx context.Context, by uuid.UUID, in Input) (Campaign, error) {
	if in.Kind != KindFestive && in.Kind != KindAnnouncement {
		return Campaign{}, ErrBadKind
	}
	in.Subject, in.Body = strings.TrimSpace(in.Subject), strings.TrimSpace(in.Body)
	f := map[string]string{}
	if n := len([]rune(in.Subject)); n < 3 || n > subjectMax {
		f["subject"] = "must be 3 to 150 characters"
	}
	if n := len([]rune(in.Body)); n < 10 || n > bodyMax {
		f["body"] = "must be 10 to 4000 characters"
	}
	if len(f) > 0 {
		return Campaign{}, apperr.Validation(f)
	}
	at := s.now()
	if in.SendAt != nil && in.SendAt.After(at) {
		at = *in.SendAt
	}
	return s.store.insert(ctx, s.pool, in.Kind, in.Subject, in.Body, nil, at, &by)
}

// List returns recent campaigns and how many guests currently receive them.
func (s *Service) List(ctx context.Context, limit int) ([]Campaign, int, error) {
	list, err := s.store.list(ctx, s.pool, limit)
	if err != nil {
		return nil, 0, err
	}
	n, err := s.store.subscribers(ctx, s.pool)
	return list, n, err
}

// Dispatch fans due campaigns out into stored emails, a batch per transaction, and returns how
// many it queued. Safe in several workers at once (SKIP LOCKED, and each guest at most once).
func (s *Service) Dispatch(ctx context.Context) (int, error) {
	total := 0
	for {
		var queued int
		var done bool
		err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			c, err := s.store.lockDue(ctx, tx, s.now())
			if errors.Is(err, pgx.ErrNoRows) {
				done = true
				return nil
			}
			if err != nil {
				return err
			}
			to, err := s.store.nextRecipients(ctx, tx, c.ID, fanOutBatch)
			if err != nil {
				return err
			}
			for _, r := range to {
				ok, err := s.store.deliver(ctx, tx, c, r, s.withUnsubscribe(c.Body, r.token))
				if err != nil {
					return err
				}
				if ok {
					queued++
				}
			}
			status := StatusSending
			if len(to) < fanOutBatch {
				status = StatusSent
			}
			return s.store.setStatus(ctx, tx, c.ID, status, s.now())
		})
		total += queued
		if err != nil || done {
			return total, err
		}
	}
}

func (s *Service) withUnsubscribe(body, token string) string {
	return body + "\n\nYou're getting this because you asked for offers when you booked with us. " +
		"Unsubscribe in one click: " + s.site + "/unsubscribe?token=" + token
}

// OptIn records a guest's consent to offers (the checkbox at checkout).
func (s *Service) OptIn(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	return s.store.optIn(ctx, q, userID, hex.EncodeToString(b), s.now())
}

// Unsubscribe stops offers for whoever holds the token.
func (s *Service) Unsubscribe(ctx context.Context, token string) error {
	ok, err := s.store.unsubscribe(ctx, s.pool, strings.TrimSpace(token))
	if err == nil && !ok {
		return ErrBadToken
	}
	return err
}
