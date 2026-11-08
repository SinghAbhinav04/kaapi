// Package enquiry is the one inbox: every channel's messages land here, the AI pipeline
// classifies them, rules route them to an owner, and staff edit and send reply drafts.
// Nothing is ever sent automatically.
package enquiry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/emailtpl"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
)

type Service struct {
	pool       *pgxpool.Pool
	classifier *ai.Classifier
	provider   ai.Provider // nil when no API key: drafts come from templates
	inventory  *inventory.Service
	now        func() time.Time
	mailer     Mailer // nil: replies are simulated
	links      emailtpl.Links
	wake       chan struct{}
	mu         sync.Mutex // serialises queue processing within this process
}

func NewService(pool *pgxpool.Pool, provider ai.Provider, inv *inventory.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, classifier: ai.NewClassifier(provider), provider: provider, inventory: inv, now: now,
		wake: make(chan struct{}, 1)}
}

// SetMailer connects real delivery; the API does it once at start-up.
func (s *Service) SetMailer(m Mailer) { s.mailer = m }

// SetLinks gives emails' buttons the site's address.
func (s *Service) SetLinks(l emailtpl.Links) { s.links = l }

// AIEnabled reports whether a model is configured (otherwise rules and templates only).
func (s *Service) AIEnabled() bool { return s.provider != nil }

// Intake stores a message and queues it for the AI. A repeat of the same message from
// the same sender within a day returns the existing reference instead of a duplicate.
func (s *Service) Intake(ctx context.Context, in IntakeInput) (string, error) {
	in.Text = strings.TrimSpace(in.Text)
	in.SenderEmail = strings.ToLower(strings.TrimSpace(in.SenderEmail))
	in.SenderPhone = strings.TrimSpace(in.SenderPhone)
	switch {
	case in.Text == "":
		return "", ErrEmptyMessage
	case in.SenderEmail == "" && in.SenderPhone == "":
		return "", ErrNeedsContact
	case !in.Channel.Valid():
		return "", ErrBadChannel
	}
	if len([]rune(in.Text)) > 5000 {
		in.Text = string([]rune(in.Text)[:5000])
	}
	hash := DedupeHash(in.Text)
	var ref string
	var followUp bool
	var ext External
	if in.External != nil {
		ext = *in.External
	}
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if ext.ID != "" { // the same email polled twice is the same enquiry
			err := tx.QueryRow(ctx, `SELECT e.ref FROM enquiries e WHERE e.external_id = $1
				UNION ALL SELECT e.ref FROM enquiry_messages m JOIN enquiries e ON e.id = m.enquiry_id WHERE m.external_id = $1
				LIMIT 1`, ext.ID).Scan(&ref)
			if err == nil || !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if ext.ThreadID != "" { // a reply in a thread we know joins its enquiry
			var id uuid.UUID
			err := tx.QueryRow(ctx, `SELECT id, ref FROM enquiries WHERE thread_id = $1 ORDER BY received_at LIMIT 1 FOR UPDATE`,
				ext.ThreadID).Scan(&id, &ref)
			if err == nil {
				followUp = true
				return addFollowUp(ctx, tx, id, ref, in, ext)
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		// Sender details are encrypted, so "the same sender" is matched on a keyed hash.
		sender := SenderIndex(in.SenderEmail, in.SenderPhone)
		err := tx.QueryRow(ctx, `SELECT ref FROM enquiries WHERE dedupe_hash = $1 AND sender_hash = $2
			AND received_at > now() - make_interval(secs => $3) LIMIT 1`, hash, sender, dedupeWindow.Seconds()).Scan(&ref)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var recent int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM enquiries WHERE sender_hash = $1
			AND received_at > now() - interval '1 hour'`, sender).Scan(&recent); err != nil {
			return err
		}
		// Imported mail already passed the mailbox's spam filter, and refusing it would lose it.
		if recent >= intakePerHour && ext.ID == "" {
			return ErrTooMany
		}
		for attempt := 0; attempt < refAttempts; attempt++ {
			var id uuid.UUID
			err := tx.QueryRow(ctx, `INSERT INTO enquiries (id, ref, channel, sender_name, sender_email, sender_phone, subject, raw_text, dedupe_hash,
					external_id, thread_id, rfc_message_id, sender_hash)
				SELECT $1, 'ENQ-' || (coalesce(max(substring(ref from 5)::int), $9 - 1) + 1), $2, $3, $4, $5, $6, $7, $8, $10, $11, $12, $13 FROM enquiries
				ON CONFLICT (ref) DO NOTHING RETURNING id, ref`,
				idgen.New(), in.Channel, pii.NullText{Text: nilIfEmpty(in.SenderName)}, pii.NullText{Text: nilIfEmpty(in.SenderEmail)},
				pii.NullText{Text: nilIfEmpty(in.SenderPhone)}, nilIfEmpty(in.Subject), in.Text, hash, firstRefNumber,
				nilIfEmpty(ext.ID), nilIfEmpty(ext.ThreadID), nilIfEmpty(ext.RFCMessageID), sender).Scan(&id, &ref)
			if errors.Is(err, pgx.ErrNoRows) {
				continue // another message took that number; try the next
			}
			if err != nil {
				return err
			}
			return addEvent(ctx, tx, id, domain.EventReceived, domain.ActorSystem, nil, nil, map[string]any{"channel": in.Channel})
		}
		return errors.New("enquiry: could not allocate a reference")
	})
	if err != nil {
		return "", err
	}
	if !followUp { // a follow-up is for staff to answer; the AI already read the enquiry
		s.Wake()
	}
	return ref, nil
}

// addFollowUp stores a guest's reply on an existing enquiry and reopens it: the next reply
// threads under this message, and the lane's team gets an alert.
func addFollowUp(ctx context.Context, tx pgx.Tx, id uuid.UUID, ref string, in IntakeInput, ext External) error {
	if _, err := tx.Exec(ctx, `INSERT INTO enquiry_messages (id, enquiry_id, body, external_id, rfc_message_id) VALUES ($1,$2,$3,$4,$5)`,
		idgen.New(), id, in.Text, nilIfEmpty(ext.ID), nilIfEmpty(ext.RFCMessageID)); err != nil {
		return err
	}
	var lane *domain.Lane
	if err := tx.QueryRow(ctx, `UPDATE enquiries SET status = CASE WHEN status = $2 THEN status ELSE $3 END,
			rfc_message_id = coalesce($4, rfc_message_id), updated_at = now()
		WHERE id = $1 RETURNING lane`, id, domain.EnquirySpam, domain.EnquiryNew, nilIfEmpty(ext.RFCMessageID)).Scan(&lane); err != nil {
		return err
	}
	team := domain.RoleGM
	if lane != nil {
		if r, ok := laneRole[*lane]; ok {
			team = r
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notifications (id, recipient_role, type, enquiry_id, body) VALUES ($1,$2,$3,$4,$5)`,
		idgen.New(), team, domain.NotifyGuestReply, id, fmt.Sprintf("%s: the guest replied: %s", ref, snippet(in.Text))); err != nil {
		return err
	}
	return addEvent(ctx, tx, id, domain.EventReceived, domain.ActorSystem, nil, nil, map[string]any{"channel": in.Channel, "follow_up": true})
}

// HasExternal reports whether a provider's message (a Gmail id) is already stored, as an
// enquiry or as a follow-up on one.
func (s *Service) HasExternal(ctx context.Context, externalID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM enquiries WHERE external_id = $1)
		OR EXISTS (SELECT 1 FROM enquiry_messages WHERE external_id = $1)`, externalID).Scan(&ok)
	return ok, err
}

// Wake nudges the worker to process the queue now rather than at its next tick.
func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run processes the queue until ctx ends: on every wake-up and on a slow tick, so nothing
// waits forever if a wake-up is missed or the process restarts.
func (s *Service) Run(ctx context.Context, tick time.Duration, onError func(error)) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		if _, err := s.ProcessPending(ctx, workerBatch); err != nil && ctx.Err() == nil {
			onError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-t.C:
		}
	}
}

// ProcessPending classifies, routes and drafts up to limit queued enquiries, oldest first.
// Each one is claimed with FOR UPDATE SKIP LOCKED, so several workers never collide.
func (s *Service) ProcessPending(ctx context.Context, limit int) (int, error) {
	done := 0
	for done < limit {
		processed, err := s.processOne(ctx)
		if err != nil {
			return done, err
		}
		if !processed {
			break
		}
		done++
	}
	return done, nil
}

// processOne handles the oldest queued enquiry. The model calls take seconds, so they run
// with no transaction open; the write then re-checks the enquiry is still queued, so if
// two workers ever pick the same one, only the first result is kept.
func (s *Service) processOne(ctx context.Context) (bool, error) {
	s.mu.Lock() // one enquiry at a time per process keeps us inside the model's rate limit
	defer s.mu.Unlock()
	var (
		id         uuid.UUID
		text       string
		senderName pii.NullText
		status     domain.EnquiryStatus
	)
	err := s.pool.QueryRow(ctx, `SELECT id, raw_text, sender_name, status FROM enquiries WHERE ai_status = $1
		ORDER BY received_at LIMIT 1`, domain.AIPending).Scan(&id, &text, &senderName, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	res, err := s.classifier.Classify(ctx, text, deref(senderName.Text), s.today())
	if err != nil {
		return true, err
	}
	lane, priority := Route(res)
	owner, err := ownerFor(ctx, s.pool, lane)
	if err != nil {
		return true, err
	}
	wantsDraft := status == domain.EnquiryNew || status == domain.EnquiryInProgress
	var p prose
	if wantsDraft {
		if p, err = s.writeProse(ctx, s.pool, text, deref(senderName.Text), res.Category, res.Extracted, lane); err != nil {
			return true, err
		}
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var queued bool
		err := tx.QueryRow(ctx, `SELECT true FROM enquiries WHERE id = $1 AND ai_status = $2 FOR UPDATE`, id, domain.AIPending).Scan(&queued)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // someone else finished it
		}
		if err != nil {
			return err
		}
		extracted, _ := json.Marshal(toStored(res.Extracted))
		if _, err := tx.Exec(ctx, `UPDATE enquiries SET language = $2, category = $3, secondary_category = $4, confidence = $5,
			extracted = $6, summary = $7, lane = $8, assigned_user_id = $9, priority = $10, routed_by = $11, ai_status = $12,
			ai_model = $13, prompt_version = $14, ai_latency_ms = $15, updated_at = now() WHERE id = $1`,
			id, nilIfEmpty(res.Language), res.Category, res.Secondary, round3(res.Confidence), extracted, nilIfEmpty(res.Summary),
			lane, owner, priority, domain.RoutedByRule, res.Status, nilIfEmpty(res.Model), ai.PromptVersion, res.LatencyMS); err != nil {
			return err
		}
		actor := domain.ActorAI
		if res.Status == domain.AIFallback {
			actor = domain.ActorSystem
		}
		if err := addEvent(ctx, tx, id, domain.EventClassified, actor, nil, nil, map[string]any{
			"category": res.Category, "confidence": round3(res.Confidence), "lane": lane, "reason": res.Reason,
			"keyword_said": res.KeywordSaid, "ai_status": res.Status}); err != nil {
			return err
		}
		if err := s.alerts(ctx, tx, id, res, lane); err != nil {
			return err
		}
		if !wantsDraft {
			return nil
		}
		// On a retry, don't bury a staff member's edit or a reply already sent.
		var author *string
		var sent *time.Time
		err = tx.QueryRow(ctx, `SELECT author, sent_at FROM enquiry_drafts WHERE enquiry_id = $1 ORDER BY version DESC LIMIT 1`, id).
			Scan(&author, &sent)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if sent != nil || (author != nil && *author == "USER") {
			return nil
		}
		next, err := nextVersion(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.storeDraft(ctx, tx, id, next, p, text, deref(senderName.Text), res.Category, res.Extracted)
	})
	if err != nil && ctx.Err() == nil {
		// Park it as FAILED so one bad message can't block the queue; staff can retry it.
		if _, ferr := s.pool.Exec(ctx, `UPDATE enquiries SET ai_status = $2, updated_at = now() WHERE id = $1`, id, domain.AIFailed); ferr != nil {
			return true, errors.Join(err, ferr)
		}
	}
	return true, err
}

// Route is a rule table, not a model decision: the category picks the lane, an unsure
// classification goes to Unsorted for the GM, and any hint of a complaint goes to the GM.
func Route(r ai.Result) (domain.Lane, domain.Priority) { return RouteWith(r, ai.UnsortedBelow) }

// RouteWith is Route with a chosen Unsorted threshold; the evaluation sweeps it.
func RouteWith(r ai.Result, unsortedBelow float64) (domain.Lane, domain.Priority) {
	if r.ComplaintHit {
		return domain.LaneGM, domain.PriorityHigh
	}
	if r.Confidence < unsortedBelow {
		return domain.LaneUnsorted, domain.PriorityNormal
	}
	return LaneFor(r.Category), domain.PriorityNormal
}

// LaneFor is where a category belongs when the classification is trusted.
func LaneFor(c domain.Category) domain.Lane {
	switch c {
	case domain.CategoryRoom:
		return domain.LaneReservations
	case domain.CategoryWedding, domain.CategoryOffsite:
		return domain.LaneSales
	case domain.CategoryComplaint:
		return domain.LaneGM
	default:
		return domain.LaneUnsorted
	}
}

// laneRole says who works each lane; Unsorted belongs to the GM.
var laneRole = map[domain.Lane]domain.Role{
	domain.LaneReservations: domain.RoleReservations,
	domain.LaneSales:        domain.RoleSales,
	domain.LaneGM:           domain.RoleGM,
	domain.LaneUnsorted:     domain.RoleGM,
}

func ownerFor(ctx context.Context, tx querier, lane domain.Lane) (*uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE role = $1 AND is_active ORDER BY created_at LIMIT 1`, laneRole[lane]).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (s *Service) alerts(ctx context.Context, tx pgx.Tx, id uuid.UUID, r ai.Result, lane domain.Lane) error {
	var ref, snippet string
	var fresh, complaintAlerted, unsortedAlerted bool
	if err := tx.QueryRow(ctx, `SELECT ref, left(raw_text, 120), received_at > now() - make_interval(secs => $2),
			EXISTS (SELECT 1 FROM notifications WHERE enquiry_id = $1 AND type = $3),
			EXISTS (SELECT 1 FROM notifications WHERE enquiry_id = $1 AND type = $4)
		FROM enquiries WHERE id = $1`, id, alertEmailWindow.Seconds(), domain.NotifyComplaintAlert, domain.NotifyUnsorted).
		Scan(&ref, &snippet, &fresh, &complaintAlerted, &unsortedAlerted); err != nil {
		return err
	}
	switch {
	case r.ComplaintHit:
		if complaintAlerted { // asking the AI again doesn't alert twice
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notifications (id, recipient_role, type, enquiry_id, body) VALUES ($1,$2,$3,$4,$5)`,
			idgen.New(), domain.RoleGM, domain.NotifyComplaintAlert, id, fmt.Sprintf("Complaint %s: %s", ref, snippet)); err != nil {
			return err
		}
		if !fresh {
			// A complaint that arrived long ago (a backlog after an outage, seeded data) is in
			// the console's alerts, but emailing people about it now would be noise.
			return addEvent(ctx, tx, id, domain.EventAlerted, domain.ActorSystem, nil, nil, map[string]any{"to": domain.RoleGM, "emailed": 0})
		}
		// Every active GM and administrator is emailed, queued for the worker to deliver (a
		// reserved test address is recorded as simulated there; before, the copy was never
		// queued, so no alert email was ever sent). Addresses are copied as stored: they are
		// already ciphertext under the same key.
		rows, err := tx.Query(ctx, `SELECT id FROM users WHERE role = ANY($1) AND is_active ORDER BY created_at`, alertRoles)
		if err != nil {
			return err
		}
		recipients, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		body := pii.Text(fmt.Sprintf("A guest complaint arrived and is waiting in the GM lane:\n\n\"%s\"\n\nOpen it in the inbox to reply.", snippet))
		for _, user := range recipients {
			if _, err := tx.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, status, recipient, subject, body, enquiry_id)
				SELECT $1, $2, $3, $4, email, $5, $6, $7 FROM users WHERE id = $8`,
				idgen.New(), domain.OutboxEmail, domain.OutboxAlert, domain.OutboxQueued, "Complaint needs you: "+ref, body, id, user); err != nil {
				return err
			}
		}
		return addEvent(ctx, tx, id, domain.EventAlerted, domain.ActorSystem, nil, nil, map[string]any{"to": alertRoles, "emailed": len(recipients)})
	case lane == domain.LaneUnsorted && !unsortedAlerted:
		_, err := tx.Exec(ctx, `INSERT INTO notifications (id, recipient_role, type, enquiry_id, body) VALUES ($1,$2,$3,$4,$5)`,
			idgen.New(), domain.RoleGM, domain.NotifyUnsorted, id, fmt.Sprintf("%s needs sorting: %s", ref, snippet))
		return err
	}
	return nil
}

func (s *Service) today() time.Time {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	t := s.now().In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ---- helpers

func addEvent(ctx context.Context, tx pgx.Tx, enquiryID uuid.UUID, typ domain.EnquiryEventType, actor domain.Actor,
	actorID *uuid.UUID, from, to map[string]any) error {
	var f, t []byte
	if from != nil {
		f, _ = json.Marshal(from)
	}
	if to != nil {
		t, _ = json.Marshal(to)
	}
	_, err := tx.Exec(ctx, `INSERT INTO enquiry_events (id, enquiry_id, type, actor, actor_user_id, from_value, to_value)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, idgen.New(), enquiryID, typ, actor, actorID, nullJSON(f), nullJSON(t))
	return err
}

// DedupeHash ignores case, spacing and punctuation, so "Is a villa free?" and
// "is a villa free" are the same message. The seeder uses it too.
func DedupeHash(text string) []byte {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	h := sha256.Sum256([]byte(b.String()))
	return h[:]
}

func nilIfEmpty(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullJSON(b []byte) any {
	if b == nil {
		return nil
	}
	return b
}

func round3(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }

// SenderIndex is the blind index that identifies a sender (their email, else their phone),
// for intake's dedupe and per-sender cap; the seeder uses it too.
func SenderIndex(email, phone string) string {
	return pii.Index(senderIndexKind, coalesceStr(strings.TrimSpace(email), strings.TrimSpace(phone)))
}
