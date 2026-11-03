package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/payment"
	"kaapi/api/internal/platform/config"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// withMailbox starts the API with Gmail connected (to the fake Google) from the environment.
func withMailbox(t *testing.T) (*apitest.Server, *fakeGoogle) {
	t.Helper()
	fake := &fakeGoogle{}
	google := httptest.NewServer(fake.handler(t))
	t.Cleanup(google.Close)
	cfg := apitest.TestConfig()
	cfg.Gmail = config.Gmail{ClientID: "client", ClientSecret: "secret", RedirectURL: "http://localhost:3000", PollEvery: time.Hour,
		RefreshToken: "refresh-1", OAuthBaseURL: google.URL, APIBaseURL: google.URL}
	s := apitest.NewWith(t, cfg)
	if err := s.App.Gmail.Bootstrap(s.Ctx); err != nil {
		t.Fatal(err)
	}
	return s, fake
}

func deliver(t *testing.T, s *apitest.Server) outbox.DeliverResult {
	t.Helper()
	r, err := s.App.Outbox.Deliver(s.Ctx, outbox.DeliverBatch)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// sentTo returns the raw messages the fake mailbox delivered to one address.
func sentTo(f *fakeGoogle, addr string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, m := range f.sent {
		if strings.Contains(m["raw"], "To: "+addr) {
			out = append(out, m["raw"])
		}
	}
	return out
}

func outboxStatus(t *testing.T, s *apitest.Server, kind domain.OutboxKind, to string) domain.OutboxStatus {
	t.Helper()
	m := outboxTo(t, s, kind, to)
	if len(m) == 0 {
		t.Fatalf("no %s to %s", kind, to)
	}
	return m[0].Status
}

func TestGuestEmailsGoOutThroughTheMailbox(t *testing.T) {
	s, fake := withMailbox(t)
	const newGuest = "first.timer@example.test"

	// Sign-in code: queued with the request, delivered by the worker.
	var sent api.OTPRequestResponse
	s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: newGuest}, nil).JSON(t, &sent)
	if st := outboxStatus(t, s, domain.OutboxOTP, newGuest); st != domain.OutboxQueued {
		t.Fatalf("before the worker runs: %s", st)
	}
	if r := deliver(t, s); r.Sent != 1 {
		t.Fatalf("delivery: %+v", r)
	}
	mails := sentTo(fake, newGuest)
	if len(mails) != 1 || !strings.Contains(mails[0], *sent.DemoCode) {
		t.Fatalf("the code must arrive by email: %v", mails)
	}
	if outboxStatus(t, s, domain.OutboxOTP, newGuest) != domain.OutboxSent {
		t.Fatal("the outbox copy should say it was sent")
	}

	// First sign-in is registration: one welcome email, never again on later sign-ins.
	s.Do(http.MethodPost, "/v1/auth/otp/verify", api.OTPVerifyRequest{Email: newGuest, Code: *sent.DemoCode}, nil)
	deliver(t, s)
	guestLogin(t, s.NewSession(), newGuest)
	deliver(t, s)
	welcomes := 0
	for _, m := range sentTo(fake, newGuest) {
		if strings.Contains(m, "Welcome to Kaapi Estate") {
			welcomes++
		}
	}
	if welcomes != 1 {
		t.Fatalf("welcome emails: %d, want exactly 1", welcomes)
	}

	// Booking: the confirmation goes out once the payment confirms it.
	b := book(t, s, holdVilla(t, s))
	pay(t, s, b.Payment.OrderID, payment.OutcomeSuccess)
	deliver(t, s)
	confirmed := false
	for _, m := range sentTo(fake, guest.Email) {
		confirmed = confirmed || strings.Contains(m, "Booking confirmed: "+b.Ref)
	}
	if !confirmed {
		t.Fatal("no booking confirmation email")
	}
	if r := deliver(t, s); r.Total() != 0 {
		t.Fatalf("nothing may be sent twice: %+v", r)
	}
}

func TestGuestEmailsAreSimulatedWithoutAMailbox(t *testing.T) {
	s := apitest.New(t)
	s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: "nobody.yet@example.test"}, nil)
	if r := deliver(t, s); r.Simulated != 1 || r.Sent != 0 {
		t.Fatalf("%+v", r)
	}
	if st := outboxStatus(t, s, domain.OutboxOTP, "nobody.yet@example.test"); st != domain.OutboxSimulated {
		t.Fatalf("status %s", st)
	}
}

func TestFailedSendsRetryThenGiveUp(t *testing.T) {
	s, fake := withMailbox(t)
	fake.mu.Lock()
	fake.failSend = true
	fake.mu.Unlock()
	const to = "unlucky@example.test"
	s.Do(http.MethodPost, "/v1/auth/otp/request", api.OTPRequest{Email: to}, nil)

	if r := deliver(t, s); r.Retrying != 1 {
		t.Fatalf("first failure should retry later: %+v", r)
	}
	id := outboxTo(t, s, domain.OutboxOTP, to)[0].ID
	var due bool
	if err := s.Pool.QueryRow(s.Ctx, `SELECT next_attempt_at <= now() FROM outbox_messages WHERE id = $1`, id).Scan(&due); err != nil || due {
		t.Fatalf("a retry must wait (backoff), due=%v err=%v", due, err)
	}
	if r := deliver(t, s); r.Total() != 0 {
		t.Fatalf("nothing is due yet: %+v", r)
	}

	// Four failures in, the fifth is the last try.
	if _, err := s.Pool.Exec(s.Ctx, `UPDATE outbox_messages SET attempts = 4, next_attempt_at = now() WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if r := deliver(t, s); r.Failed != 1 {
		t.Fatalf("fifth failure: %+v", r)
	}
	var reason string
	if err := s.Pool.QueryRow(s.Ctx, `SELECT last_error FROM outbox_messages WHERE id = $1`, id).Scan(&reason); err != nil || reason == "" {
		t.Fatalf("the failure reason should be kept for staff: %q %v", reason, err)
	}
	if outboxStatus(t, s, domain.OutboxOTP, to) != domain.OutboxFailed {
		t.Fatal("status should be FAILED")
	}
}
