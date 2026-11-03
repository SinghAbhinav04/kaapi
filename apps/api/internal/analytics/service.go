// Package analytics answers "how is direct booking doing": revenue, pace, mix and what's
// already sold for the months ahead. Read-only; every number comes from bookings, nights
// and inventory, never from anything the AI wrote.
package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
	now   func() time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{pool: pool, now: now}
}

// Today is the resort's calendar date, for callers choosing a default range.
func (s *Service) Today() time.Time { return dateIn(s.now(), ist()) }

// Report covers bookings made from `from` to `to` (IST calendar dates, both inclusive).
func (s *Service) Report(ctx context.Context, from, to time.Time) (Report, error) {
	loc := ist()
	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	r := Report{From: from, To: to}

	var err error
	if r.KPIs, err = s.store.kpis(ctx, s.pool, start, end); err != nil {
		return r, err
	}
	r.KPIs.derive()
	// The same number of days just before, for the trend arrows.
	prevStart := start.Add(-end.Sub(start))
	if r.KPIs.PreviousRevenuePaise, r.KPIs.PreviousBookingsCount, err = s.store.previous(ctx, s.pool, prevStart, start); err != nil {
		return r, err
	}
	if r.Daily, err = s.store.daily(ctx, s.pool, from, to, start, end); err != nil {
		return r, err
	}
	if r.ByRoom, err = s.store.breakdown(ctx, s.pool, `rt.name`, `room_types rt LEFT JOIN bookings b ON b.room_type_id = rt.id`, start, end); err != nil {
		return r, err
	}
	if r.ByPlan, err = s.store.breakdown(ctx, s.pool, `rp.name`, `rate_plans rp LEFT JOIN bookings b ON b.rate_plan_id = rp.id`, start, end); err != nil {
		return r, err
	}
	if r.ByCurrency, err = s.store.byCurrency(ctx, s.pool, start, end); err != nil {
		return r, err
	}
	if r.StatusMix, err = s.store.counts(ctx, s.pool, `SELECT status, count(*) FROM bookings WHERE created_at >= $1 AND created_at < $2
		GROUP BY 1 ORDER BY 2 DESC`, start, end); err != nil {
		return r, err
	}
	counts, err := s.store.leadCounts(ctx, s.pool, start, end, leadThresholds())
	if err != nil {
		return r, err
	}
	r.LeadTime = make([]Count, len(leadBuckets))
	for i, b := range leadBuckets {
		r.LeadTime[i] = Count{Key: b.Label, Count: counts[i]}
	}
	if r.OnTheBooks, err = s.store.onTheBooks(ctx, s.pool, dateIn(s.now(), loc)); err != nil {
		return r, err
	}
	r.Enquiries, err = s.store.enquiries(ctx, s.pool, start, end)
	return r, err
}

// derive fills the ratios from the raw totals.
func (k *KPIs) derive() {
	if k.RoomNights > 0 {
		k.ADRPaise = k.RoomRevenuePaise / int64(k.RoomNights)
	}
	if k.Bookings > 0 {
		k.AvgBookingValuePaise = k.RevenuePaise / int64(k.Bookings)
	}
	if n := k.Bookings + k.cancellations; n > 0 {
		rate := float64(k.cancellations) / float64(n)
		k.CancellationRate = &rate
	}
	k.CommissionSavedPaise = k.RoomRevenuePaise * agencyCommissionBps / bpsDenominator
}

// leadThresholds is each bucket's lower bound after the first, for width_bucket.
func leadThresholds() []int {
	t := make([]int, 0, len(leadBuckets)-1)
	for _, b := range leadBuckets[1:] {
		t = append(t, b.Min)
	}
	return t
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
