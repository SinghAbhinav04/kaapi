package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
)

// store is every query the report runs. Read-only.
type store struct{}

var (
	confirmed = string(domain.BookingConfirmed)
	// Paid for, then called off or given back.
	cancelled = []string{string(domain.BookingCancelled), string(domain.BookingRefundPending), string(domain.BookingRefunded)}
)

// previous is revenue and bookings for the same-length window before, for the trend arrows.
func (store) previous(ctx context.Context, q db.Querier, start, end time.Time) (revenue int64, bookings int, err error) {
	err = q.QueryRow(ctx, `SELECT COALESCE(sum(total_paise), 0), count(*) FROM bookings
		WHERE status = $1 AND created_at >= $2 AND created_at < $3`, confirmed, start, end).Scan(&revenue, &bookings)
	return revenue, bookings, err
}

func (st store) kpis(ctx context.Context, q db.Querier, start, end time.Time) (KPIs, error) {
	var k KPIs
	var cancels int
	err := q.QueryRow(ctx, `SELECT
			COALESCE(sum(total_paise) FILTER (WHERE status = $3), 0),
			COALESCE(sum(subtotal_paise - discount_paise) FILTER (WHERE status = $3), 0),
			COALESCE(sum(tax_paise) FILTER (WHERE status = $3), 0),
			count(*) FILTER (WHERE status = $3),
			COALESCE(sum(nights * rooms) FILTER (WHERE status = $3), 0),
			COALESCE(avg(nights) FILTER (WHERE status = $3), 0)::float8,
			COALESCE(avg(check_in - (created_at AT TIME ZONE 'Asia/Kolkata')::date) FILTER (WHERE status = $3), 0)::float8,
			count(*) FILTER (WHERE status = ANY($4))
		FROM bookings WHERE created_at >= $1 AND created_at < $2`, start, end, confirmed, cancelled).
		Scan(&k.RevenuePaise, &k.RoomRevenuePaise, &k.GSTPaise, &k.Bookings, &k.RoomNights, &k.AvgStayNights, &k.AvgLeadDays, &cancels)
	if err != nil {
		return k, err
	}
	k.cancellations = cancels
	return k, nil
}

// daily has one row per calendar day, zeros included, so the chart's x-axis is honest.
func (st store) daily(ctx context.Context, q db.Querier, from, to, start, end time.Time) ([]Day, error) {
	rows, err := q.Query(ctx, `SELECT d::date, COALESCE(x.n, 0), COALESCE(x.rev, 0)
		FROM generate_series($1::date, $2::date, interval '1 day') d
		LEFT JOIN (SELECT (created_at AT TIME ZONE 'Asia/Kolkata')::date AS day, count(*) AS n, sum(total_paise) AS rev
			FROM bookings WHERE status = $3 AND created_at >= $4 AND created_at < $5 GROUP BY 1) x ON x.day = d::date
		ORDER BY 1`, from, to, confirmed, start, end)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Day, error) {
		var d Day
		err := r.Scan(&d.Date, &d.Bookings, &d.RevenuePaise)
		return d, err
	})
}

// breakdown groups confirmed bookings by a catalogue name, listing every name even with none.
func (st store) breakdown(ctx context.Context, q db.Querier, name, from string, start, end time.Time) ([]Breakdown, error) {
	// name and from are constants in this file; the booking filter lives in the join so
	// catalogue rows without bookings still appear.
	rows, err := q.Query(ctx, `SELECT `+name+`, count(b.id), COALESCE(sum(b.nights * b.rooms), 0), COALESCE(sum(b.total_paise), 0)
		FROM `+from+` AND b.status = $1 AND b.created_at >= $2 AND b.created_at < $3
		GROUP BY 1 ORDER BY 4 DESC, 1`, confirmed, start, end)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Breakdown, error) {
		var b Breakdown
		err := r.Scan(&b.Name, &b.Bookings, &b.RoomNights, &b.RevenuePaise)
		return b, err
	})
}

// leadCounts counts confirmed bookings per lead-time bucket in the database, so only one
// row per bucket comes back however many bookings there are. thresholds are the lower
// bounds of every bucket after the first (width_bucket returns how many a value reaches).
func (st store) leadCounts(ctx context.Context, q db.Querier, start, end time.Time, thresholds []int) ([]int, error) {
	rows, err := q.Query(ctx, `SELECT width_bucket(check_in - (created_at AT TIME ZONE 'Asia/Kolkata')::date, $4::int[]) AS bucket, count(*)
		FROM bookings WHERE status = $1 AND created_at >= $2 AND created_at < $3 GROUP BY 1`, confirmed, start, end, thresholds)
	if err != nil {
		return nil, err
	}
	counts := make([]int, len(thresholds)+1)
	for rows.Next() {
		var bucket, n int
		if err := rows.Scan(&bucket, &n); err != nil {
			rows.Close()
			return nil, err
		}
		counts[min(max(bucket, 0), len(thresholds))] += n
	}
	return counts, rows.Err()
}

// onTheBooks: for this month and the next few, nights already sold and their room revenue
// (each booking's revenue spread evenly over its nights), against the website's allotment.
func (st store) onTheBooks(ctx context.Context, q db.Querier, today time.Time) ([]Month, error) {
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, AheadMonths, 0)
	rows, err := q.Query(ctx, `WITH
		m AS (SELECT generate_series($1::date, $2::date - interval '1 month', interval '1 month')::date AS month),
		inv AS (SELECT date_trunc('month', date)::date AS month, sum(allotment) AS a, sum(sold) AS s
			FROM night_inventory WHERE date >= $1 AND date < $2 GROUP BY 1),
		bn AS (SELECT date_trunc('month', n.date)::date AS month, sum(n.rooms) AS rn,
				sum((b.subtotal_paise - b.discount_paise) / b.nights) AS rev
			FROM booking_nights n JOIN bookings b ON b.id = n.booking_id
			WHERE b.status = $3 AND n.date >= $1 AND n.date < $2 GROUP BY 1)
		SELECT m.month, COALESCE(bn.rn, 0), COALESCE(bn.rev, 0)::bigint, COALESCE(inv.a, 0), COALESCE(inv.s, 0)
		FROM m LEFT JOIN inv USING (month) LEFT JOIN bn USING (month) ORDER BY 1`, first, last, confirmed)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Month, error) {
		var m Month
		err := r.Scan(&m.Month, &m.RoomNights, &m.RoomRevenuePaise, &m.Allotment, &m.Sold)
		return m, err
	})
}

func (st store) enquiries(ctx context.Context, q db.Querier, start, end time.Time) (Enquiries, error) {
	var e Enquiries
	var done, fallback int
	err := q.QueryRow(ctx, `SELECT count(*), count(linked_booking_id),
			(percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM first_response_at - received_at) / 60))::float8,
			count(*) FILTER (WHERE ai_status = $3), count(*) FILTER (WHERE ai_status = $4)
		FROM enquiries WHERE received_at >= $1 AND received_at < $2`,
		start, end, string(domain.AIDone), string(domain.AIFallback)).
		Scan(&e.Total, &e.Converted, &e.MedianFirstReplyMinutes, &done, &fallback)
	if err != nil {
		return e, err
	}
	if n := done + fallback; n > 0 {
		rate := float64(done) / float64(n)
		e.AIHandledRate = &rate
	}
	if e.ByChannel, err = st.counts(ctx, q, `SELECT channel, count(*) FROM enquiries WHERE received_at >= $1 AND received_at < $2
		GROUP BY 1 ORDER BY 2 DESC, 1`, start, end); err != nil {
		return e, err
	}
	e.ByCategory, err = st.counts(ctx, q, `SELECT COALESCE(category, 'UNREAD'), count(*) FROM enquiries
		WHERE received_at >= $1 AND received_at < $2 GROUP BY 1 ORDER BY 2 DESC, 1`, start, end)
	return e, err
}

func (st store) counts(ctx context.Context, q db.Querier, sql string, args ...any) ([]Count, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Count, error) {
		var c Count
		err := r.Scan(&c.Key, &c.Count)
		return c, err
	})
}
