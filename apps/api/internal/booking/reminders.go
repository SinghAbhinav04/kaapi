package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
)

// Reminders: two about arriving and two about leaving, each sent exactly once per booking.
// The job only *checks* every few minutes; booking_reminders' primary key means a guest never
// gets the same reminder twice. Timings are labelled assumptions:
//   - CHECKIN_SOON: once, when check-in is 1 to 3 days away
//   - CHECKIN_TODAY: once, on the morning of check-in (from 8 am IST)
//   - CHECKOUT_SOON: once, two days before check-out, if they've already arrived
//   - CHECKOUT_TODAY: once, on the morning of check-out (7 to 11 am IST, before check-out time)
const (
	reminderCheckInSoon   = "CHECKIN_SOON"
	reminderCheckInToday  = "CHECKIN_TODAY"
	reminderCheckOutSoon  = "CHECKOUT_SOON"
	reminderCheckOutToday = "CHECKOUT_TODAY"

	checkInSoonDays     = 3
	checkOutSoonDays    = 2
	checkInTodayFrom    = 8  // IST hour
	checkOutTodayFrom   = 7  // IST hour
	checkOutTodayBefore = 11 // check-out time: no point reminding after it
	reminderBatch       = 100
)

// QueueReminders queues every reminder that's due now and returns how many.
func (s *Service) QueueReminders(ctx context.Context) (int, error) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	now := s.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	hour := now.Hour()

	due := []struct {
		kind  string
		ok    bool
		write func(context.Context, pgx.Tx, uuid.UUID) error
	}{
		{reminderCheckInSoon, true, s.checkInSoon},
		{reminderCheckInToday, hour >= checkInTodayFrom, s.checkInToday},
		{reminderCheckOutSoon, true, s.checkOutSoon},
		{reminderCheckOutToday, hour >= checkOutTodayFrom && hour < checkOutTodayBefore, s.checkOutToday},
	}
	total := 0
	for _, d := range due {
		if !d.ok {
			continue
		}
		n, err := s.queueReminders(ctx, d.kind, today, d.write)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (s *Service) queueReminders(ctx context.Context, kind string, today time.Time,
	write func(context.Context, pgx.Tx, uuid.UUID) error) (int, error) {
	total := 0
	for {
		var queued int
		var last bool
		err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			ids, err := s.store.dueReminders(ctx, tx, kind, today, reminderBatch)
			if err != nil {
				return err
			}
			queued, last = 0, len(ids) < reminderBatch
			for _, id := range ids {
				fresh, err := s.store.markReminded(ctx, tx, id, kind)
				if err != nil {
					return err
				}
				if !fresh {
					continue // another worker queued it a moment ago
				}
				if err := write(ctx, tx, id); err != nil {
					return err
				}
				queued++
			}
			return nil
		})
		if err != nil {
			return total, err
		}
		total += queued
		if last {
			return total, nil
		}
	}
}

func (s *Service) checkInSoon(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	return s.remind(ctx, tx, id, domain.OutboxCheckInReminder, "See you soon at Kaapi Estate", func(b Booking) string {
		return fmt.Sprintf("Hi %s, we're looking forward to welcoming you on %s.\n\n"+
			"%s, %s\n%s to %s (%d nights), %d room(s)\n\n"+
			"Check-in is from 2 pm. The last 10 km up the hill have patchy signal, so download the map before you set off. "+
			"Kadur railway station is about 40 km away: reply to this email if you'd like us to arrange a pick-up.\n\n"+
			"Guests from outside India: please bring your passport and visa for check-in.",
			firstName(b.Guest.Name), b.CheckIn.Format("Monday 2 January"), b.RoomTypeName, b.RatePlanName,
			b.CheckIn.Format("Mon 2 Jan"), b.CheckOut.Format("Mon 2 Jan 2006"), b.Nights(), b.Rooms)
	})
}

func (s *Service) checkInToday(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	return s.remind(ctx, tx, id, domain.OutboxCheckInReminder, "Welcome day: your room is ready from 2 pm", func(b Booking) string {
		return fmt.Sprintf("Good morning, %s. Today's the day: your %s is ready from 2 pm.\n\n"+
			"Signal fades in the last 10 km, so open the directions before you lose it. If you're running late, just reply and we'll keep everything ready.",
			firstName(b.Guest.Name), b.RoomTypeName)
	})
}

func (s *Service) checkOutSoon(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	return s.remind(ctx, tx, id, domain.OutboxCheckOutReminder, "Two more nights with us", func(b Booking) string {
		return fmt.Sprintf("Hi %s, a note that your stay ends on %s, with check-out by 11 am.\n\n"+
			"Want a little longer? Reply to this email and we'll check whether we can extend your stay or arrange a late check-out.",
			firstName(b.Guest.Name), b.CheckOut.Format("Monday 2 January"))
	})
}

func (s *Service) checkOutToday(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	return s.remind(ctx, tx, id, domain.OutboxCheckOutReminder, "Check-out today by 11 am", func(b Booking) string {
		return fmt.Sprintf("Good morning, %s. A gentle reminder that check-out is by 11 am today.\n\n"+
			"Leave your key at the front desk; your stay was paid when you booked, so there's nothing to settle unless you added anything. "+
			"Thank you for staying with us. We hope the estate was everything you hoped for.",
			firstName(b.Guest.Name))
	})
}

// remind queues one reminder email for a booking, in the job's transaction.
func (s *Service) remind(ctx context.Context, tx pgx.Tx, id uuid.UUID, kind domain.OutboxKind, subject string, body func(Booking) string) error {
	b, err := s.store.one(ctx, tx, `b.id = $1`, id)
	if err != nil {
		return err
	}
	return s.store.queueEmail(ctx, tx, kind, b.Guest.Email, subject+": "+b.Ref, body(b), id)
}
