package notify

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/platform/db"
)

// store is every query this package runs.
type store struct{}

// visible is the alerts a viewer may see: their team's, their own, or all for an administrator.
const visible = `($1 OR n.recipient_role = $2 OR n.recipient_user_id = $3)`

// addressed is the alerts meant for the viewer: their team's or their own (unread counts the
// same). Read state is shared by the team, so an administrator looking over every team's
// alerts counts and clears only their own; marking a GM's complaint read stays the GM's call.
const addressed = `(n.recipient_role = $2 OR n.recipient_user_id = $3)`

func (store) list(ctx context.Context, q db.Querier, v Viewer, limit int) ([]Notification, error) {
	rows, err := q.Query(ctx, `SELECT n.id, n.type, n.body, e.ref, b.ref, n.created_at, n.read_at
		FROM notifications n
		LEFT JOIN enquiries e ON e.id = n.enquiry_id
		LEFT JOIN bookings b ON b.id = n.booking_id
		WHERE `+visible+` ORDER BY n.created_at DESC LIMIT $4`, v.seesAll(), v.Team, v.User, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Notification, error) {
		var n Notification
		return n, r.Scan(&n.ID, &n.Type, &n.Body, &n.EnquiryRef, &n.BookingRef, &n.CreatedAt, &n.ReadAt)
	})
}

func (store) unread(ctx context.Context, q db.Querier, v Viewer) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM notifications n
		WHERE (n.recipient_role = $1 OR n.recipient_user_id = $2) AND n.read_at IS NULL`, v.Team, v.User).Scan(&n)
	return n, err
}

// markRead marks one alert (id set) or every one addressed to the viewer (id nil) read. It
// reports how many visible alerts matched, so an unknown or hidden id can be told apart; an
// administrator opening another team's alert sees it without marking it read for them.
func (store) markRead(ctx context.Context, q db.Querier, v Viewer, id *uuid.UUID) (int64, error) {
	tag, err := q.Exec(ctx, `WITH hit AS (
			SELECT n.id, `+addressed+` AS mine FROM notifications n
			WHERE `+visible+` AND ($4::uuid IS NULL OR n.id = $4) AND ($4::uuid IS NOT NULL OR n.read_at IS NULL))
		UPDATE notifications n SET read_at = CASE WHEN hit.mine THEN coalesce(n.read_at, now()) ELSE n.read_at END
		FROM hit WHERE n.id = hit.id AND (hit.mine OR $4::uuid IS NOT NULL)`,
		v.seesAll(), v.Team, v.User, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
