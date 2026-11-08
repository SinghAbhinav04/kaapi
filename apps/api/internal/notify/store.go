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
	err := q.QueryRow(ctx, `SELECT count(*) FROM notifications n WHERE `+visible+` AND n.read_at IS NULL`,
		v.seesAll(), v.Team, v.User).Scan(&n)
	return n, err
}

// markRead marks one alert (id set) or every visible one (id nil) read; it reports how many
// visible alerts matched, so an unknown or hidden id can be told apart.
func (store) markRead(ctx context.Context, q db.Querier, v Viewer, id *uuid.UUID) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE notifications n SET read_at = coalesce(n.read_at, now())
		WHERE `+visible+` AND ($4::uuid IS NULL OR n.id = $4) AND ($4::uuid IS NOT NULL OR n.read_at IS NULL)`,
		v.seesAll(), v.Team, v.User, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
