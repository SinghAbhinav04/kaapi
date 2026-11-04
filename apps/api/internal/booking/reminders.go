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

// Reminder timing (assumptions, labelled in the TDD): a pre-arrival email once the stay is
// within two days, and a check-out email from 7 am IST on the day they leave.
const (
	reminderCheckIn      = "CHECKIN"
	reminderCheckOut     = "CHECKOUT"
	reminderLeadDays     = 2
	checkOutReminderHour = 7
	reminderBatch        = 100
)

// QueueReminders queues every reminder that's due, each exactly once (booking_reminders'
// primary key), however often and in however many workers it runs. Returns how many.
func (s *Service) QueueReminders(ctx context.Context) (int, error) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	now := s.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	n, err := s.queueReminders(ctx, reminderCheckIn, today, today.AddDate(0, 0, reminderLeadDays), s.checkInReminder)
	if err != nil || now.Hour() < checkOutReminderHour {
		return n, err
	}
	m, err := s.queueReminders(ctx, reminderCheckOut, today, today, s.checkOutReminder)
	return n + m, err
}

func (s *Service) queueReminders(ctx context.Context, kind string, from, to time.Time,
	write func(context.Context, pgx.Tx, uuid.UUID) error) (int, error) {
	total := 0
	for {
		var queued int
		var last bool
		err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			ids, err := s.store.dueReminders(ctx, tx, kind, from, to, reminderBatch)
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

func (s *Service) checkInReminder(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	b, err := s.store.one(ctx, tx, `b.id = $1`, id)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Hi %s, we're looking forward to welcoming you on %s.\n\n"+
		"%s, %s\n%s to %s (%d nights), %d room(s)\n\n"+
		"Check-in is from 2 pm and check-out by 11 am. The last 10 km up the hill have patchy signal, so download the map before you set off. "+
		"Kadur railway station is about 40 km away: reply to this email if you'd like us to arrange a pick-up.\n\n"+
		"Guests from outside India: please bring your passport and visa for check-in.",
		firstName(b.Guest.Name), b.CheckIn.Format("Monday 2 January"), b.RoomTypeName, b.RatePlanName,
		b.CheckIn.Format("Mon 2 Jan"), b.CheckOut.Format("Mon 2 Jan 2006"), b.Nights(), b.Rooms)
	return s.store.queueEmail(ctx, tx, domain.OutboxCheckInReminder, b.Guest.Email, "See you soon at Kaapi Estate: "+b.Ref, body, id)
}

func (s *Service) checkOutReminder(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	b, err := s.store.one(ctx, tx, `b.id = $1`, id)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Good morning, %s. A gentle reminder that check-out is by 11 am today.\n\n"+
		"Leave your key at the front desk; your bill was settled when you booked, so there's nothing to pay unless you added anything during your stay. "+
		"If you'd like a little longer, reply to this email and we'll try to arrange a late check-out.\n\n"+
		"Thank you for staying with us. We hope the estate was everything you hoped for.",
		firstName(b.Guest.Name))
	return s.store.queueEmail(ctx, tx, domain.OutboxCheckOutReminder, b.Guest.Email, "Check-out today: "+b.Ref, body, id)
}
