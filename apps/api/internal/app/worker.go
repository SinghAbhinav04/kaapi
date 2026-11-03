package app

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/gmail"
	"kaapi/api/internal/platform/apperr"
)

// Worker timings.
const (
	// Unpaid bookings are expired this long after their hold ends; late payments inside
	// the grace period still go through if the room is free.
	sweepEvery = time.Minute
	sweepGrace = 30 * time.Minute

	// The enquiry worker wakes on every new message; this tick is only a safety net.
	enquiryTick = 15 * time.Second

	// Retention windows are months long, so a few runs a day is plenty.
	retentionEvery = 6 * time.Hour
)

// RunWorkers runs every background job until ctx ends: the AI queue, the Gmail poller, the
// unpaid-booking sweep and the retention sweep. Production runs it in its own process
// (cmd/worker) so the API can scale to zero; local development runs it inside cmd/api.
// Every job is safe to run in more than one process at once (SKIP LOCKED, idempotent sweeps).
func (a *App) RunWorkers(ctx context.Context) {
	jobs := []func(context.Context){
		func(ctx context.Context) {
			a.Enquiries.Run(ctx, enquiryTick, func(err error) { a.Log.Error("enquiry worker", "err", err) })
		},
		a.Gmail.Run,
		func(ctx context.Context) {
			every(ctx, sweepEvery, false, func() {
				if n, err := a.Bookings.Sweep(ctx, sweepGrace); err != nil {
					a.Log.Error("booking sweep failed", "err", err)
				} else if n > 0 {
					a.Log.Info("expired unpaid bookings", "count", n)
				}
			})
		},
		func(ctx context.Context) {
			every(ctx, retentionEvery, true, func() {
				if r, err := a.Retention.Sweep(ctx); err != nil {
					a.Log.Error("retention sweep failed", "err", err)
				} else if r.Total() > 0 {
					a.Log.Info("retention sweep", "result", r)
				}
			})
		},
	}
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Go(func() { job(ctx) })
	}
	wg.Wait()
}

// ProcessEnquiries runs the AI queue now; tests use it instead of waiting for the worker.
func (a *App) ProcessEnquiries(ctx context.Context) (int, error) {
	return a.Enquiries.ProcessPending(ctx, enquiry.MaxListLimit)
}

// every runs fn on a ticker (and once at the start when now is set) until ctx ends.
func every(ctx context.Context, d time.Duration, now bool, fn func()) {
	if now {
		fn()
	}
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
		}
	}
}

// mailInbox is the enquiry inbox as the Gmail importer sees it.
type mailInbox struct{ *App }

func (m mailInbox) Has(ctx context.Context, messageID string) (bool, error) {
	return m.Enquiries.HasExternal(ctx, messageID)
}

// Deliver turns an imported email into an enquiry. A message the inbox refuses for a reason
// that won't change on retry (no text, bad channel) is skipped, not retried forever.
func (m mailInbox) Deliver(ctx context.Context, in gmail.Inbound) error {
	a := m.App
	_, err := a.Enquiries.Intake(ctx, enquiry.IntakeInput{Channel: domain.ChannelEmail, SenderName: in.FromName,
		SenderEmail: in.FromEmail, Subject: in.Subject, Text: in.Text,
		External: &enquiry.External{ID: in.MessageID, ThreadID: in.ThreadID, RFCMessageID: in.RFCMessageID}})
	var ae *apperr.Error
	if errors.As(err, &ae) && ae.Status() < http.StatusInternalServerError {
		a.Log.Warn("gmail message skipped", "id", in.MessageID, "reason", ae.Message)
		return nil
	}
	return err
}
