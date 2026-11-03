package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
)

// store is every query this package runs. Read-only.
type store struct{}

const roomSelect = `SELECT id, slug, name, kind, size_sqm, bed, view, base_occupancy, max_adults, max_children,
	extra_adult_paise, summary, description, amenities FROM room_types`

func (store) activeRooms(ctx context.Context, q db.Querier) ([]RoomType, error) {
	rows, err := q.Query(ctx, roomSelect+` WHERE is_active ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRoom)
}

// roomBySlug returns pgx.ErrNoRows when there is no such active room.
func (store) roomBySlug(ctx context.Context, q db.Querier, slug string) (RoomType, error) {
	rows, err := q.Query(ctx, roomSelect+` WHERE is_active AND slug = $1`, slug)
	if err != nil {
		return RoomType{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanRoom)
}

func scanRoom(r pgx.CollectableRow) (RoomType, error) {
	var x RoomType
	return x, r.Scan(&x.ID, &x.Slug, &x.Name, &x.Kind, &x.SizeSqm, &x.Bed, &x.View, &x.BaseOccupancy, &x.MaxAdults,
		&x.MaxChildren, &x.ExtraAdultPaise, &x.Summary, &x.Description, &x.Amenities)
}

// attach loads photos and plans for the given rooms in two queries, not two per room.
func (store) attach(ctx context.Context, q db.Querier, rooms []RoomType) error {
	ids := make([]uuid.UUID, len(rooms))
	index := make(map[uuid.UUID]int, len(rooms))
	for i, r := range rooms {
		ids[i], index[r.ID] = r.ID, i
	}
	rows, err := q.Query(ctx, `SELECT room_type_id, url, alt FROM room_type_photos WHERE room_type_id = ANY($1) ORDER BY sort`, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		var p Photo
		if err := rows.Scan(&id, &p.URL, &p.Alt); err != nil {
			rows.Close()
			return err
		}
		rooms[index[id]].Photos = append(rooms[index[id]].Photos, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	rows, err = q.Query(ctx, `SELECT room_type_id, id, code, name, inclusions, refundable, cancellation_policy, min_nights
		FROM rate_plans WHERE room_type_id = ANY($1) AND is_active ORDER BY sort`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var p Plan
		if err := rows.Scan(&id, &p.ID, &p.Code, &p.Name, &p.Inclusions, &p.Refundable, &p.CancellationPolicy, &p.MinNights); err != nil {
			return err
		}
		rooms[index[id]].Plans = append(rooms[index[id]].Plans, p)
	}
	return rows.Err()
}

// planID returns pgx.ErrNoRows when the room or plan doesn't exist or isn't on sale.
func (store) planID(ctx context.Context, q db.Querier, slug, planCode string) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT rp.id FROM rate_plans rp JOIN room_types rt ON rt.id = rp.room_type_id
		WHERE rt.slug = $1 AND rp.code = $2 AND rp.is_active AND rt.is_active`, slug, planCode).Scan(&id)
	return id, err
}

// calendar is each night's price and rooms left (after live holds). "Season" compares each
// night with the median price for its weekday in the range: a cheap way to mark Diwali or
// Christmas nights without the client knowing the rules.
func (store) calendar(ctx context.Context, q db.Querier, planID uuid.UUID, from, to time.Time) ([]CalendarDay, error) {
	rows, err := q.Query(ctx, `
		WITH nights AS (
			SELECT ni.date, nr.price_paise,
			       ni.allotment - ni.sold - coalesce(h.held, 0) AS left,
			       ni.min_stay, ni.stop_sell, ni.closed_to_arrival
			FROM night_inventory ni
			JOIN rate_plans rp ON rp.room_type_id = ni.room_type_id AND rp.id = $1
			JOIN night_rates nr ON nr.rate_plan_id = rp.id AND nr.date = ni.date
			LEFT JOIN LATERAL (
				SELECT sum(rooms)::int AS held FROM holds
				WHERE room_type_id = ni.room_type_id AND status = $4 AND expires_at > now()
				  AND check_in <= ni.date AND check_out > ni.date
			) h ON true
			WHERE ni.date >= $2 AND ni.date < $3
		)
		SELECT n.date, n.price_paise, greatest(n.left, 0), n.min_stay, n.stop_sell, n.closed_to_arrival,
		       n.price_paise > percentile_disc(0.5) WITHIN GROUP (ORDER BY n2.price_paise)
		FROM nights n JOIN nights n2 ON extract(isodow FROM n2.date) = extract(isodow FROM n.date)
		GROUP BY n.date, n.price_paise, n.left, n.min_stay, n.stop_sell, n.closed_to_arrival
		ORDER BY n.date`, planID, from, to, domain.HoldActive)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (CalendarDay, error) {
		var d CalendarDay
		return d, r.Scan(&d.Date, &d.PricePaise, &d.Left, &d.MinStay, &d.StopSell, &d.NoArrival, &d.Season)
	})
}

func (store) property(ctx context.Context, q db.Querier) (Property, error) {
	var p Property
	err := q.QueryRow(ctx, `SELECT name, to_char(check_in_time, 'HH24:MI'), to_char(check_out_time, 'HH24:MI'), hold_minutes
		FROM property_settings`).Scan(&p.Name, &p.CheckInTime, &p.CheckOutTime, &p.HoldMinutes)
	if err != nil {
		return p, fmt.Errorf("property settings: %w", err)
	}
	rows, err := q.Query(ctx, `SELECT key, category, question, answer FROM kb_facts WHERE is_active ORDER BY category, key`)
	if err != nil {
		return p, err
	}
	p.Facts, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Fact, error) {
		var f Fact
		return f, r.Scan(&f.Key, &f.Category, &f.Question, &f.Answer)
	})
	return p, err
}
