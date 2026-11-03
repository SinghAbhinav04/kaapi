// Package analytics answers "how is direct booking doing": revenue, pace, mix and what's
// already sold for the months ahead. Read-only; every number comes from bookings, nights
// and inventory, never from anything the AI wrote.
package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{pool: pool, now: now}
}

var (
	confirmed = string(domain.BookingConfirmed)
	// Paid for, then called off or given back.
	cancelled = []string{string(domain.BookingCancelled), string(domain.BookingRefundPending), string(domain.BookingRefunded)}
)

// Today is the resort's calendar date, for callers choosing a default range.
func (s *Service) Today() time.Time { return dateIn(s.now(), ist()) }

// Report covers bookings made from `from` to `to` (IST calendar dates, both inclusive).
func (s *Service) Report(ctx context.Context, from, to time.Time) (Report, error) {
	loc := ist()
	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	r := Report{From: from, To: to}

	var err error
	if r.KPIs, err = s.kpis(ctx, start, end); err != nil {
		return r, err
	}
	// The same number of days just before, for the trend arrows.
	prevStart := start.Add(-end.Sub(start))
	if err = s.pool.QueryRow(ctx, `SELECT COALESCE(sum(total_paise), 0), count(*) FROM bookings
		WHERE status = $1 AND created_at >= $2 AND created_at < $3`, confirmed, prevStart, start).
		Scan(&r.KPIs.PreviousRevenuePaise, &r.KPIs.PreviousBookingsCount); err != nil {
		return r, err
	}
	if r.Daily, err = s.daily(ctx, from, to, start, end); err != nil {
		return r, err
	}
	if r.ByRoom, err = s.breakdown(ctx, `rt.name`, `room_types rt LEFT JOIN bookings b ON b.room_type_id = rt.id`, start, end); err != nil {
		return r, err
	}
	if r.ByPlan, err = s.breakdown(ctx, `rp.name`, `rate_plans rp LEFT JOIN bookings b ON b.rate_plan_id = rp.id`, start, end); err != nil {
		return r, err
	}
	if r.ByCurrency, err = s.byCurrency(ctx, start, end); err != nil {
		return r, err
	}
	if r.StatusMix, err = s.counts(ctx, `SELECT status, count(*) FROM bookings WHERE created_at >= $1 AND created_at < $2
		GROUP BY 1 ORDER BY 2 DESC`, start, end); err != nil {
		return r, err
	}
	if r.LeadTime, err = s.leadTime(ctx, start, end); err != nil {
		return r, err
	}
	if r.OnTheBooks, err = s.onTheBooks(ctx, dateIn(s.now(), loc)); err != nil {
		return r, err
	}
	r.Enquiries, err = s.enquiries(ctx, start, end)
	return r, err
}

func (s *Service) kpis(ctx context.Context, start, end time.Time) (KPIs, error) {
	var k KPIs
	var cancels int
	err := s.pool.QueryRow(ctx, `SELECT
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
	if k.RoomNights > 0 {
		k.ADRPaise = k.RoomRevenuePaise / int64(k.RoomNights)
	}
	if k.Bookings > 0 {
		k.AvgBookingValuePaise = k.RevenuePaise / int64(k.Bookings)
	}
	if n := k.Bookings + cancels; n > 0 {
		rate := float64(cancels) / float64(n)
		k.CancellationRate = &rate
	}
	k.CommissionSavedPaise = k.RoomRevenuePaise * agencyCommissionBps / bpsDenominator
	return k, nil
}

// daily has one row per calendar day, zeros included, so the chart's x-axis is honest.
func (s *Service) daily(ctx context.Context, from, to, start, end time.Time) ([]Day, error) {
	rows, err := s.pool.Query(ctx, `SELECT d::date, COALESCE(x.n, 0), COALESCE(x.rev, 0)
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
func (s *Service) breakdown(ctx context.Context, name, from string, start, end time.Time) ([]Breakdown, error) {
	// name and from are constants in this file; the booking filter lives in the join so
	// catalogue rows without bookings still appear.
	rows, err := s.pool.Query(ctx, `SELECT `+name+`, count(b.id), COALESCE(sum(b.nights * b.rooms), 0), COALESCE(sum(b.total_paise), 0)
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

// byCurrency groups confirmed bookings by the currency the guest was shown. Bookings made
// before currencies existed, or without one sent, were shown INR.
func (s *Service) byCurrency(ctx context.Context, start, end time.Time) ([]Breakdown, error) {
	rows, err := s.pool.Query(ctx, `SELECT COALESCE(display_currency, $4), count(*), COALESCE(sum(nights * rooms), 0), COALESCE(sum(total_paise), 0)
		FROM bookings WHERE status = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 4 DESC, 1`, confirmed, start, end, string(domain.CurrencyINR))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Breakdown, error) {
		var b Breakdown
		err := r.Scan(&b.Name, &b.Bookings, &b.RoomNights, &b.RevenuePaise)
		return b, err
	})
}

func (s *Service) leadTime(ctx context.Context, start, end time.Time) ([]Count, error) {
	rows, err := s.pool.Query(ctx, `SELECT check_in - (created_at AT TIME ZONE 'Asia/Kolkata')::date FROM bookings
		WHERE status = $1 AND created_at >= $2 AND created_at < $3`, confirmed, start, end)
	if err != nil {
		return nil, err
	}
	leads, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return nil, err
	}
	out := make([]Count, len(leadBuckets))
	for i, b := range leadBuckets {
		out[i].Key = b.Label
		for _, d := range leads {
			if d >= b.Min && (b.Max < 0 || d <= b.Max) {
				out[i].Count++
			}
		}
	}
	return out, nil
}

// onTheBooks: for this month and the next few, nights already sold and their room revenue
// (each booking's revenue spread evenly over its nights), against the website's allotment.
func (s *Service) onTheBooks(ctx context.Context, today time.Time) ([]Month, error) {
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, AheadMonths, 0)
	rows, err := s.pool.Query(ctx, `WITH
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

func (s *Service) enquiries(ctx context.Context, start, end time.Time) (Enquiries, error) {
	var e Enquiries
	var done, fallback int
	err := s.pool.QueryRow(ctx, `SELECT count(*), count(linked_booking_id),
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
	if e.ByChannel, err = s.counts(ctx, `SELECT channel, count(*) FROM enquiries WHERE received_at >= $1 AND received_at < $2
		GROUP BY 1 ORDER BY 2 DESC, 1`, start, end); err != nil {
		return e, err
	}
	e.ByCategory, err = s.counts(ctx, `SELECT COALESCE(category, 'UNREAD'), count(*) FROM enquiries
		WHERE received_at >= $1 AND received_at < $2 GROUP BY 1 ORDER BY 2 DESC, 1`, start, end)
	return e, err
}

func (s *Service) counts(ctx context.Context, q string, args ...any) ([]Count, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Count, error) {
		var c Count
		err := r.Scan(&c.Key, &c.Count)
		return c, err
	})
}

func ist() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return loc
}

// dateIn is t's calendar date in loc, as stay dates are stored: midnight UTC.
func dateIn(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
