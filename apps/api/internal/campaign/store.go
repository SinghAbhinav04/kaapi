package campaign

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
)

// store is every query this package runs.
type store struct{}

const campaignColumns = `id, kind, subject, body, status, send_at, sent_at, recipients, created_at`

func scanCampaign(r pgx.CollectableRow) (Campaign, error) {
	var c Campaign
	return c, r.Scan(&c.ID, &c.Kind, &c.Subject, &c.Body, &c.Status, &c.SendAt, &c.SentAt, &c.Recipients, &c.CreatedAt)
}

func (store) insert(ctx context.Context, q db.Querier, kind Kind, subject, body string, promoID *uuid.UUID, sendAt time.Time, by *uuid.UUID) (Campaign, error) {
	rows, err := q.Query(ctx, `INSERT INTO campaigns (id, kind, subject, body, promo_code_id, send_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+campaignColumns, idgen.New(), kind, subject, body, promoID, sendAt, by)
	if err != nil {
		return Campaign{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanCampaign)
}

// insertOfferCampaign announces an offer, once: a second announcement of it does nothing.
func (store) insertOfferCampaign(ctx context.Context, q db.Querier, subject, body string, promoID uuid.UUID, sendAt time.Time, by uuid.UUID) error {
	_, err := q.Exec(ctx, `INSERT INTO campaigns (id, kind, subject, body, promo_code_id, send_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (promo_code_id) WHERE kind = 'OFFER' DO NOTHING`,
		idgen.New(), KindOffer, subject, body, promoID, sendAt, by)
	return err
}

func (store) list(ctx context.Context, q db.Querier, limit int) ([]Campaign, error) {
	rows, err := q.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanCampaign)
}

func (store) subscribers(ctx context.Context, q db.Querier) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM users WHERE marketing_opt_in AND is_active AND role = $1`, domain.RoleGuest).Scan(&n)
	return n, err
}

// lockDue locks one campaign that's due and not finished; pgx.ErrNoRows when none.
func (store) lockDue(ctx context.Context, q db.Querier, now time.Time) (Campaign, error) {
	rows, err := q.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns
		WHERE status IN ($1, $2) AND send_at <= $3 ORDER BY send_at LIMIT 1 FOR UPDATE SKIP LOCKED`,
		StatusScheduled, StatusSending, now)
	if err != nil {
		return Campaign{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanCampaign)
}

// recipient is an opted-in guest who hasn't had this campaign yet.
type recipient struct {
	id    uuid.UUID
	email string
	token string
}

func (store) nextRecipients(ctx context.Context, q db.Querier, campaignID uuid.UUID, limit int) ([]recipient, error) {
	rows, err := q.Query(ctx, `SELECT u.id, u.email, u.unsubscribe_token FROM users u
		WHERE u.marketing_opt_in AND u.is_active AND u.role = $2 AND u.unsubscribe_token IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM campaign_recipients r WHERE r.campaign_id = $1 AND r.user_id = u.id)
		ORDER BY u.id LIMIT $3`, campaignID, domain.RoleGuest, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (recipient, error) {
		var x recipient
		return x, r.Scan(&x.id, pii.Into(&x.email), &x.token)
	})
}

// deliver records the recipient and queues their email in one step; false if already done.
func (store) deliver(ctx context.Context, q db.Querier, c Campaign, to recipient, body string) (bool, error) {
	tag, err := q.Exec(ctx, `INSERT INTO campaign_recipients (campaign_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, c.ID, to.id)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	_, err = q.Exec(ctx, `INSERT INTO outbox_messages (id, channel, kind, status, recipient, subject, body) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		idgen.New(), domain.OutboxEmail, domain.OutboxPromotion, domain.OutboxQueued, pii.Text(to.email), c.Subject, pii.Text(body))
	return err == nil, err
}

func (store) setStatus(ctx context.Context, q db.Querier, id uuid.UUID, status Status, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE campaigns SET status = $2,
		sent_at = CASE WHEN $2 = 'SENT' THEN $3 ELSE sent_at END,
		recipients = (SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1) WHERE id = $1`, id, status, at)
	return err
}

// optIn records a guest's consent to offers, giving them an unsubscribe token once.
func (store) optIn(ctx context.Context, q db.Querier, userID uuid.UUID, token string, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET marketing_opt_in = true, marketing_opt_in_at = coalesce(marketing_opt_in_at, $3),
		unsubscribe_token = coalesce(unsubscribe_token, $2) WHERE id = $1 AND role = $4`, userID, token, at, domain.RoleGuest)
	return err
}

// unsubscribe reports false for an unknown token.
func (store) unsubscribe(ctx context.Context, q db.Querier, token string) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE users SET marketing_opt_in = false WHERE unsubscribe_token = $1`, token)
	return err == nil && tag.RowsAffected() == 1, err
}
