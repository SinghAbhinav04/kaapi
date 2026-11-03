package enquiry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
)

// List returns inbox rows, complaints first, then newest.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Summary, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Lanes != nil { // nil: every lane; empty: none (a team with no lane of its own)
		add("e.lane = ANY($%d)", f.Lanes)
	}
	if f.Status != "" {
		add("e.status = $%d", f.Status)
	} else {
		add("e.status <> ALL($%d)", []domain.EnquiryStatus{domain.EnquiryClosed, domain.EnquirySpam})
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		// Same expression as enquiries_search_trgm_idx, so the trigram index serves it. Sender
		// names are encrypted at rest, so search covers the reference and the message.
		add("(e.ref || ' ' || e.raw_text) ILIKE '%%' || $%d || '%%'", q)
	}
	args = append(args, ai.UnsortedBelow, f.Limit)
	cond := strings.Join(where, " AND ")
	// Urgent (HIGH and NEW) first, then newest. Ordering by that expression directly forces a
	// sort over every open enquiry; two index-ordered halves that each stop at the limit don't.
	// Only the start of each message is read: the list shows a snippet.
	q := fmt.Sprintf(`WITH picked AS (
			(SELECT e.id, true AS urgent, e.received_at FROM enquiries e
				WHERE %[1]s AND e.priority = 'HIGH' AND e.status = 'NEW' ORDER BY e.received_at DESC LIMIT $%[3]d)
			UNION ALL
			(SELECT e.id, false, e.received_at FROM enquiries e
				WHERE %[1]s AND NOT (e.priority = 'HIGH' AND e.status = 'NEW') ORDER BY e.received_at DESC LIMIT $%[3]d))
		SELECT e.ref, e.channel, e.sender_name, left(e.raw_text, %[4]d), e.received_at, e.category, e.confidence::float8, e.lane,
			e.status, e.priority, e.ai_status, e.category = 'ROOM' AND (e.extracted->>'check_in' IS NULL),
			e.confidence < $%[2]d, EXISTS (SELECT 1 FROM enquiry_drafts d WHERE d.enquiry_id = e.id)
		FROM picked p JOIN enquiries e ON e.id = p.id
		ORDER BY p.urgent DESC, p.received_at DESC LIMIT $%[3]d`,
		cond, len(args)-1, len(args), snippetSourceChars)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Summary, error) {
		var x Summary
		var text string
		var noDates, unsure *bool
		err := r.Scan(&x.Ref, &x.Channel, pii.IntoNull(&x.SenderName), &text, &x.ReceivedAt, &x.Category, &x.Confidence, &x.Lane,
			&x.Status, &x.Priority, &x.AIStatus, &noDates, &unsure, &x.HasDraft)
		x.Snippet = snippet(text)
		x.NeedsCheck = (noDates != nil && *noDates) || (unsure != nil && *unsure)
		return x, err
	})
}

// Get returns one enquiry with its latest draft and history.
func (s *Service) Get(ctx context.Context, ref string) (Enquiry, error) {
	var e Enquiry
	var extracted []byte
	var assignedID *uuid.UUID
	var assignedName *string
	var assignedRole *domain.Role
	err := s.pool.QueryRow(ctx, `SELECT e.id, e.ref, e.channel, e.sender_name, e.sender_email, e.sender_phone, e.subject, e.raw_text,
		e.received_at, e.language, e.category, e.secondary_category, e.confidence::float8, e.extracted, e.summary, e.lane, e.priority,
		e.status, e.ai_status, e.ai_model, e.first_response_at, u.id, u.name, u.role
		FROM enquiries e LEFT JOIN users u ON u.id = e.assigned_user_id WHERE e.ref = $1`, strings.ToUpper(ref)).
		Scan(&e.ID, &e.Ref, &e.Channel, pii.IntoNull(&e.SenderName), pii.IntoNull(&e.SenderEmail), pii.IntoNull(&e.SenderPhone),
			&e.Subject, &e.Text, &e.ReceivedAt, &e.Language, &e.Category, &e.Secondary, &e.Confidence, &extracted, &e.Summary,
			&e.Lane, &e.Priority, &e.Status, &e.AIStatus, &e.AIModel, &e.FirstReply, &assignedID, pii.IntoNull(&assignedName), &assignedRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	e.Extracted = fromStored(extracted)
	if assignedID != nil {
		e.AssignedTo = &Staff{ID: *assignedID, Name: *assignedName, Role: *assignedRole}
	}

	var d Draft
	var quote []byte
	err = s.pool.QueryRow(ctx, `SELECT version, body_rendered, body_template, author, flagged_reason, quote, created_at, sent_at
		FROM enquiry_drafts WHERE enquiry_id = $1 ORDER BY version DESC LIMIT 1`, e.ID).
		Scan(&d.Version, &d.Body, &d.Template, &d.Author, &d.FlaggedReason, &quote, &d.CreatedAt, &d.SentAt)
	if err == nil {
		d.QuoteFilled = quote != nil
		e.Draft = &d
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return e, err
	}

	rows, err := s.pool.Query(ctx, `SELECT ev.type, ev.actor, u.name, ev.from_value, ev.to_value, ev.created_at
		FROM enquiry_events ev LEFT JOIN users u ON u.id = ev.actor_user_id WHERE ev.enquiry_id = $1 ORDER BY ev.created_at, ev.id`, e.ID)
	if err != nil {
		return e, err
	}
	e.Events, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Event, error) {
		var ev Event
		var from, to []byte
		err := r.Scan(&ev.Type, &ev.Actor, pii.IntoNull(&ev.ActorName), &from, &to, &ev.CreatedAt)
		_ = json.Unmarshal(from, &ev.From)
		_ = json.Unmarshal(to, &ev.To)
		return ev, err
	})
	if err != nil {
		return e, err
	}
	for _, ev := range e.Events {
		if ev.Type == domain.EventClassified {
			if r, ok := ev.To["reason"].(string); ok {
				e.AIReason = r
			}
		}
	}
	return e, nil
}

// Reroute moves an enquiry to another lane and optionally corrects its category. A changed
// category gets a fresh draft. Every correction is logged; together they measure how often
// staff disagree with the AI.
func (s *Service) Reroute(ctx context.Context, ref string, actor uuid.UUID, lane domain.Lane, category *domain.Category) error {
	if !lane.Valid() {
		return ErrBadLane
	}
	if category != nil && !category.Valid() {
		return ErrBadCategory
	}
	var oldCat *domain.Category
	var text string
	var sender *string
	var extracted []byte
	err := s.pool.QueryRow(ctx, `SELECT category, raw_text, sender_name, extracted FROM enquiries WHERE ref = $1`,
		strings.ToUpper(ref)).Scan(&oldCat, &text, pii.IntoNull(&sender), &extracted)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	owner, err := ownerFor(ctx, s.pool, lane)
	if err != nil {
		return err
	}
	recategorised := category != nil && (oldCat == nil || *oldCat != *category)
	var p prose
	if recategorised { // the new draft's wording is written before the transaction opens
		if p, err = s.writeProse(ctx, s.pool, text, deref(sender), *category, fromStored(extracted), owner); err != nil {
			return err
		}
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id uuid.UUID
		var oldLane domain.Lane
		var lockedCat *domain.Category
		err := tx.QueryRow(ctx, `SELECT id, lane, category FROM enquiries WHERE ref = $1 FOR UPDATE`, strings.ToUpper(ref)).
			Scan(&id, &oldLane, &lockedCat)
		if err != nil {
			return err
		}
		newCat := lockedCat
		if category != nil {
			newCat = category
		}
		if _, err := tx.Exec(ctx, `UPDATE enquiries SET lane = $2, category = $3, assigned_user_id = $4, routed_by = $5, updated_at = now()
			WHERE id = $1`, id, lane, newCat, owner, domain.RoutedByHuman); err != nil {
			return err
		}
		if oldLane != lane {
			if err := addEvent(ctx, tx, id, domain.EventRerouted, domain.ActorUser, &actor,
				map[string]any{"lane": oldLane}, map[string]any{"lane": lane}); err != nil {
				return err
			}
		}
		if !recategorised {
			return nil
		}
		if err := addEvent(ctx, tx, id, domain.EventCategoryChanged, domain.ActorUser, &actor,
			map[string]any{"category": lockedCat}, map[string]any{"category": category}); err != nil {
			return err
		}
		next, err := nextVersion(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.storeDraft(ctx, tx, id, next, p, text, deref(sender), *category, fromStored(extracted))
	})
}

// Correct fixes what the AI extracted and re-renders the draft's quote with the new details.
func (s *Service) Correct(ctx context.Context, ref string, actor uuid.UUID, c Corrections) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id uuid.UUID
		var raw []byte
		var text string
		var sender *string
		var cat *domain.Category
		var owner *uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id, extracted, raw_text, sender_name, category, assigned_user_id FROM enquiries WHERE ref = $1 FOR UPDATE`,
			strings.ToUpper(ref)).Scan(&id, &raw, &text, pii.IntoNull(&sender), &cat, &owner)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		before := fromStored(raw)
		after := before
		changed := false
		set := func(dst **time.Time, v *time.Time) {
			if v != nil {
				*dst, changed = v, true
			}
		}
		setInt := func(dst **int, v *int) {
			if v != nil {
				*dst, changed = v, true
			}
		}
		set(&after.CheckIn, c.CheckIn)
		set(&after.CheckOut, c.CheckOut)
		setInt(&after.Adults, c.Adults)
		setInt(&after.Children, c.Children)
		setInt(&after.Rooms, c.Rooms)
		setInt(&after.GuestCount, c.GuestCount)
		if !changed {
			return ErrNothingToSave
		}
		if after.CheckIn != nil && after.CheckOut != nil && !after.CheckOut.After(*after.CheckIn) {
			return apperr.Validation(map[string]string{"check_out": "must be after check-in"})
		}
		stored, _ := json.Marshal(toStored(after))
		if _, err := tx.Exec(ctx, `UPDATE enquiries SET extracted = $2, updated_at = now() WHERE id = $1`, id, stored); err != nil {
			return err
		}
		if err := addEvent(ctx, tx, id, domain.EventFieldsCorrected, domain.ActorUser, &actor,
			anyMap(toStored(before)), anyMap(toStored(after))); err != nil {
			return err
		}
		// Re-render the latest unsent draft with the corrected details (prose unchanged).
		// A staff member's own text has no placeholders, so it is left exactly as they wrote it.
		var template, author string
		var version int
		var sent *time.Time
		err = tx.QueryRow(ctx, `SELECT body_template, author, version, sent_at FROM enquiry_drafts WHERE enquiry_id = $1 ORDER BY version DESC LIMIT 1`, id).
			Scan(&template, &author, &version, &sent)
		if errors.Is(err, pgx.ErrNoRows) || sent != nil || cat == nil || author == "USER" {
			return nil
		}
		if err != nil {
			return err
		}
		staff, err := staffFor(ctx, tx, owner)
		if err != nil {
			return err
		}
		rendered, quote, err := s.render(ctx, tx, template, text, deref(sender), *cat, after, staff)
		if err != nil {
			return err
		}
		return s.saveDraft(ctx, tx, id, version+1, template, rendered, quote, "AI", nil, "")
	})
}

// EditDraft stores the staff member's text as the next version.
func (s *Service) EditDraft(ctx context.Context, ref string, actor uuid.UUID, body string) error {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > maxDraftRunes {
		return apperr.Validation(map[string]string{"body": fmt.Sprintf("must be 1 to %d characters", maxDraftRunes)})
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := lockByRef(ctx, tx, ref)
		if err != nil {
			return err
		}
		next, err := nextVersion(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.saveDraft(ctx, tx, id, next, body, body, nil, "USER", &actor, "")
	})
}

// Send delivers the latest draft. It's the only way anything reaches a guest, and it always
// needs a person to press it. With a mailbox connected an email reply goes out for real, in
// the guest's thread; otherwise (and for WhatsApp) it's recorded in the outbox as a simulation.
//
// Real delivery never holds a transaction across the network call: the draft is claimed in
// one transaction, sent, then marked sent in another. The claim stops a double click sending
// twice; if delivery fails the claim is released and nothing is marked sent.
func (s *Service) Send(ctx context.Context, ref string, actor uuid.UUID) error {
	useMail := s.mailer != nil && s.mailer.Connected(ctx)
	var p sendPlan
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if p, err = loadSendPlan(ctx, tx, ref); err != nil {
			return err
		}
		if !useMail || p.channel != domain.OutboxEmail {
			p.simulated = true
			return p.record(ctx, tx, actor, domain.OutboxSimulated, domain.SentMail{})
		}
		tag, err := tx.Exec(ctx, `UPDATE enquiry_drafts SET sending_at = now() WHERE id = $1 AND sent_at IS NULL
			AND (sending_at IS NULL OR sending_at < now() - make_interval(secs => $2))`, p.draftID, sendClaimTTL.Seconds())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrSending
		}
		return nil
	})
	if err != nil || p.simulated {
		return err
	}

	sent, err := s.mailer.Send(ctx, domain.Mail{To: p.recipient, Subject: p.subject, Body: p.body, InReplyTo: p.inReplyTo, ThreadID: p.threadID})
	if err != nil {
		// Detached context: release the claim even if the request was cancelled.
		_, _ = s.pool.Exec(context.WithoutCancel(ctx), `UPDATE enquiry_drafts SET sending_at = NULL WHERE id = $1`, p.draftID)
		return err
	}
	// The guest has the email now; record it even if the request was cancelled meanwhile.
	detached := context.WithoutCancel(ctx)
	return db.WithTx(detached, s.pool, func(tx pgx.Tx) error {
		return p.record(detached, tx, actor, domain.OutboxSent, sent)
	})
}

// sendPlan is everything Send needs, read under the enquiry's row lock.
type sendPlan struct {
	enquiryID, draftID       uuid.UUID
	version                  int
	channel                  domain.OutboxChannel
	recipient, subject, body string
	inReplyTo, threadID      string
	simulated                bool
}

func loadSendPlan(ctx context.Context, tx pgx.Tx, ref string) (sendPlan, error) {
	var p sendPlan
	var channel domain.Channel
	var email, phone, subject, thread, msgID *string
	err := tx.QueryRow(ctx, `SELECT id, channel, sender_email, sender_phone, subject, thread_id, rfc_message_id
		FROM enquiries WHERE ref = $1 FOR UPDATE`, strings.ToUpper(ref)).
		Scan(&p.enquiryID, &channel, pii.IntoNull(&email), pii.IntoNull(&phone), &subject, &thread, &msgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	var sent *time.Time
	err = tx.QueryRow(ctx, `SELECT id, body_rendered, sent_at, version FROM enquiry_drafts WHERE enquiry_id = $1 ORDER BY version DESC LIMIT 1`,
		p.enquiryID).Scan(&p.draftID, &p.body, &sent, &p.version)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNoDraft
	}
	if err != nil {
		return p, err
	}
	if sent != nil {
		return p, ErrAlreadySent
	}
	p.channel, p.recipient = domain.OutboxEmail, deref(email)
	if channel == domain.ChannelWhatsApp || p.recipient == "" {
		p.channel, p.recipient = domain.OutboxWhatsApp, deref(phone)
	}
	if p.recipient == "" {
		return p, ErrNoRecipient
	}
	p.subject = "Re: your enquiry to Kaapi Estate"
	if subject != nil && *subject != "" {
		p.subject = *subject
		if !strings.HasPrefix(strings.ToLower(p.subject), "re:") {
			p.subject = "Re: " + p.subject
		}
	}
	p.threadID, p.inReplyTo = deref(thread), deref(msgID)
	return p, nil
}

// record writes the outbox copy, marks the draft sent and the enquiry replied. For a real
// delivery it only succeeds if the draft is still unsent, which the claim guarantees.
func (p sendPlan) record(ctx context.Context, tx pgx.Tx, actor uuid.UUID, status domain.OutboxStatus, sent domain.SentMail) error {
	tag, err := tx.Exec(ctx, `UPDATE enquiry_drafts SET sent_at = now(), sent_by = $2, sending_at = NULL WHERE id = $1 AND sent_at IS NULL`,
		p.draftID, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadySent
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, recipient, subject, body, enquiry_id, sent_by, status, external_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, idgen.New(), p.channel, domain.OutboxEnquiryReply, pii.Text(p.recipient), p.subject, pii.Text(p.body),
		p.enquiryID, actor, status, nilIfEmpty(sent.MessageID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE enquiries SET status = $2, first_response_at = coalesce(first_response_at, now()),
		thread_id = coalesce(thread_id, $3), updated_at = now() WHERE id = $1`, p.enquiryID, domain.EnquiryReplied, nilIfEmpty(sent.ThreadID)); err != nil {
		return err
	}
	return addEvent(ctx, tx, p.enquiryID, domain.EventSent, domain.ActorUser, &actor, nil,
		map[string]any{"channel": p.channel, "version": p.version, "delivery": status})
}

// SetStatus closes, reopens or marks an enquiry as spam.
func (s *Service) SetStatus(ctx context.Context, ref string, actor uuid.UUID, status domain.EnquiryStatus) error {
	if !status.Valid() {
		return apperr.Validation(map[string]string{"status": "is not an enquiry status"})
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := lockByRef(ctx, tx, ref)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE enquiries SET status = $2, updated_at = now() WHERE id = $1`, id, status); err != nil {
			return err
		}
		return addEvent(ctx, tx, id, domain.EventClosed, domain.ActorUser, &actor, nil, map[string]any{"status": status})
	})
}

// RetryAI queues an enquiry for classification again (after an outage, or a new model).
func (s *Service) RetryAI(ctx context.Context, ref string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE enquiries SET ai_status = $2, updated_at = now() WHERE ref = $1`, strings.ToUpper(ref), domain.AIPending)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.Wake()
	return nil
}

// LaneCounts counts the enquiries the inbox lists by default (everything but closed and spam)
// in each lane; nil lanes means every lane. The sidebar shows these, so it needs two small
// queries instead of the whole inbox.
func (s *Service) LaneCounts(ctx context.Context, lanes []domain.Lane) (map[domain.Lane]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT lane, count(*) FROM enquiries
		WHERE status <> ALL($1) AND ($2::text[] IS NULL OR lane = ANY($2)) GROUP BY lane`,
		[]domain.EnquiryStatus{domain.EnquiryClosed, domain.EnquirySpam}, lanes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.Lane]int{}
	for rows.Next() {
		var lane domain.Lane
		var n int
		if err := rows.Scan(&lane, &n); err != nil {
			return nil, err
		}
		out[lane] = n
	}
	return out, rows.Err()
}

// MedianFirstReply is the median minutes to a first reply over the stats window (nil: none yet).
func (s *Service) MedianFirstReply(ctx context.Context) (*float64, error) {
	var median *float64
	err := s.pool.QueryRow(ctx, `SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM first_response_at - received_at) / 60)
		FROM enquiries WHERE first_response_at IS NOT NULL AND first_response_at >= $1`, s.now().Add(-statsWindow)).Scan(&median)
	return median, err
}

// Stats are the dashboard numbers, measured against the brief's success targets.
func (s *Service) Stats(ctx context.Context) (Stats, error) {
	st := Stats{OpenByLane: map[domain.Lane]int{}}
	since := s.now().Add(-statsWindow)
	err := s.pool.QueryRow(ctx, `SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM first_response_at - received_at) / 60),
		count(*) FROM enquiries WHERE first_response_at IS NOT NULL AND first_response_at >= $1`, since).
		Scan(&st.MedianFirstReplyMinutes, &st.RepliedThisWeek)
	if err != nil {
		return st, err
	}
	rows, err := s.pool.Query(ctx, `SELECT lane, count(*) FROM enquiries WHERE status IN ($1, $2) GROUP BY lane`,
		domain.EnquiryNew, domain.EnquiryInProgress)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var lane domain.Lane
		var n int
		if err := rows.Scan(&lane, &n); err != nil {
			rows.Close()
			return st, err
		}
		st.OpenByLane[lane] = n
	}
	rows.Close()
	err = s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM enquiries WHERE category = $1 AND status IN ($2, $3)),
		(SELECT count(*) FROM enquiries WHERE ai_status = $4),
		(SELECT count(*) FROM bookings WHERE status = $5 AND confirmed_at >= $6 AND hold_id IS NOT NULL),
		(SELECT coalesce(sum(total_paise), 0) FROM bookings WHERE status = $5 AND confirmed_at >= $6 AND hold_id IS NOT NULL),
		(SELECT count(*) FROM night_inventory WHERE sold > allotment)`,
		domain.CategoryComplaint, domain.EnquiryNew, domain.EnquiryInProgress, domain.AIPending, domain.BookingConfirmed, since).
		Scan(&st.OpenComplaints, &st.AwaitingAI, &st.DirectBookingsThisWeek, &st.DirectRevenueThisWeek, &st.Overbookings)
	if err != nil {
		return st, err
	}
	var routed, corrected int
	if err := s.pool.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE type = $1), count(*) FILTER (WHERE type IN ($2, $3) AND actor = $4)
		FROM enquiry_events WHERE created_at >= $5`,
		domain.EventClassified, domain.EventRerouted, domain.EventCategoryChanged, domain.ActorUser, since).Scan(&routed, &corrected); err != nil {
		return st, err
	}
	if routed > 0 {
		r := float64(corrected) / float64(routed)
		st.AICorrectionRate = &r
	}
	return st, nil
}

// ---- helpers

func lockByRef(ctx context.Context, tx pgx.Tx, ref string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM enquiries WHERE ref = $1 FOR UPDATE`, strings.ToUpper(ref)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, ErrNotFound
	}
	return id, err
}

func nextVersion(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, error) {
	var v int
	err := tx.QueryRow(ctx, `SELECT coalesce(max(version), 0) + 1 FROM enquiry_drafts WHERE enquiry_id = $1`, id).Scan(&v)
	return v, err
}

func snippet(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) <= snippetRunes {
		return text
	}
	return string(r[:snippetRunes-1]) + "…"
}

func anyMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
