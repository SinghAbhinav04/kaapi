// Package retention archives finished bookings, strips guest contact details once they are
// no longer needed, and deletes what the privacy policy says we don't keep. It runs as a
// periodic job; every step is idempotent and works in small batches with SKIP LOCKED, so it
// never blocks bookings or the inbox and can be re-run at any time. The policy lives here;
// the SQL lives in store.go.
package retention

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/platform/db"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
	now   func() time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{pool: pool, now: now}
}

type picker func(context.Context, db.Querier) ([]uuid.UUID, error)
type actor func(context.Context, db.Querier, []uuid.UUID) error

// Sweep runs every rule once and records what it did in the audit log.
func (s *Service) Sweep(ctx context.Context) (Result, error) {
	var r Result
	now := s.now()
	today := todayIST(now)
	st := s.store
	messagesBefore := now.AddDate(0, -messageRetention, 0)
	deleteFrom := func(table string) actor {
		return func(ctx context.Context, q db.Querier, ids []uuid.UUID) error { return st.deleteFrom(ctx, q, table, ids) }
	}
	at := func(t time.Time, pick func(context.Context, db.Querier, time.Time) ([]uuid.UUID, error)) picker {
		return func(ctx context.Context, q db.Querier) ([]uuid.UUID, error) { return pick(ctx, q, t) }
	}

	steps := []struct {
		count *int64
		pick  picker
		act   actor
	}{
		// Stays that are over leave the live list but keep every link.
		{&r.Archived, at(today, st.pickToArchive), func(ctx context.Context, q db.Querier, ids []uuid.UUID) error { return st.archive(ctx, q, ids, now) }},
		// Contact details go 12 months after check-out; the tax record stays.
		{&r.Anonymised, at(today.AddDate(0, -contactRetention, 0), st.pickToAnonymise),
			func(ctx context.Context, q db.Querier, ids []uuid.UUID) error { return st.anonymise(ctx, q, ids, now) }},
		// After the tax window the booking itself goes.
		{&r.PurgedTax, at(PurgeBefore(today, TaxRetentionFY), st.pickPastTaxWindow), st.deleteBookings},
		{&r.PurgedUnpaid, at(now.Add(-unpaidRetention), st.pickUnpaid), st.deleteBookings},
		// Enquiry events, drafts and alerts cascade; outbox copies lose the link and go next.
		{&r.Enquiries, at(messagesBefore, st.pickEnquiries), deleteFrom("enquiries")},
		{&r.Outbox, at(messagesBefore, st.pickOutbox), deleteFrom("outbox_messages")},
		{&r.Notifications, at(messagesBefore, st.pickNotifications), deleteFrom("notifications")},
		{&r.Holds, at(now.Add(-holdRetention), st.pickHolds), deleteFrom("holds")},
		{&r.OTPs, at(now.Add(-otpRetention), st.pickOTPs), deleteFrom("otp_codes")},
		{&r.Sessions, at(now.Add(-sessionRetention), st.pickSessions), st.deleteSessions},
	}
	for _, step := range steps {
		n, err := s.batches(ctx, step.pick, step.act)
		*step.count = n
		if err != nil {
			return r, err
		}
	}
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		r.Housekeeping, err = st.housekeeping(ctx, tx, now.Add(-bucketRetention), now)
		return err
	})
	if err != nil {
		return r, err
	}
	if r.Total() > 0 {
		err = st.audit(ctx, s.pool, today, r)
	}
	return r, err
}

// batches picks and acts in one short transaction per batch, until a batch comes back short.
func (s *Service) batches(ctx context.Context, pick picker, act actor) (int64, error) {
	var total int64
	for {
		var n int
		err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			ids, err := pick(ctx, tx)
			if n = len(ids); err != nil || n == 0 {
				return err
			}
			return act(ctx, tx, ids)
		})
		if err != nil {
			return total, err
		}
		total += int64(n)
		if n < batchSize {
			return total, nil
		}
	}
}

// todayIST is the resort's calendar date, as stay dates are stored: midnight UTC.
func todayIST(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
