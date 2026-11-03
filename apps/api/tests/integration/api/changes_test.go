package api_test

import (
	"bufio"
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"kaapi/api/internal/payment"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// withChangeFeed starts the API with the analytics cache on and the change feed listening,
// as cmd/api runs it.
func withChangeFeed(t *testing.T) *apitest.Server {
	t.Helper()
	cfg := apitest.TestConfig()
	cfg.AnalyticsCacheTTL = time.Hour
	s := apitest.NewWith(t, cfg)
	ctx, cancel := context.WithCancel(s.Ctx)
	t.Cleanup(cancel)
	go s.App.Changes.Run(ctx)
	return s
}

var serverTimingQueries = regexp.MustCompile(`db;dur=[0-9.]+;desc="(\d+) queries"`)

func queriesUsed(t *testing.T, res apitest.Response) int {
	t.Helper()
	m := serverTimingQueries.FindStringSubmatch(res.Header.Get("Server-Timing"))
	if m == nil {
		t.Fatalf("no Server-Timing database figure: %q", res.Header.Get("Server-Timing"))
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

const analyticsRange = "/v1/staff/analytics?from=2026-01-01&to=2026-12-31"

func TestAnalyticsAreCachedUntilAPaymentChangesThem(t *testing.T) {
	s := withChangeFeed(t)
	staffLogin(t, s, "gm@kaapiestate.test", nil)

	var before api.AnalyticsResponse
	first := s.Do(http.MethodGet, analyticsRange, nil, nil)
	first.JSON(t, &before)
	second := s.Do(http.MethodGet, analyticsRange, nil, nil)
	if queriesUsed(t, second) >= queriesUsed(t, first) {
		t.Fatalf("the repeat should be served from the cache: %d queries, then %d", queriesUsed(t, first), queriesUsed(t, second))
	}

	// A guest pays: the triggers notify, the feed drops the cached report, the next request sees it.
	guestSide := s.NewSession()
	b := book(t, guestSide, holdVilla(t, guestSide))
	pay(t, guestSide, b.Payment.OrderID, payment.OutcomeSuccess)

	deadline := time.Now().Add(5 * time.Second)
	for {
		var after api.AnalyticsResponse
		s.Do(http.MethodGet, analyticsRange, nil, nil).JSON(t, &after)
		if after.KPIs.Bookings == before.KPIs.Bookings+1 && after.KPIs.RevenuePaise == before.KPIs.RevenuePaise+b.Quote.TotalPaise {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("analytics never reflected the payment: before %d bookings, after %d", before.KPIs.Bookings, after.KPIs.Bookings)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestChangeStreamTellsConsolesWhatChanged(t *testing.T) {
	s := withChangeFeed(t)
	login := s.Do(http.MethodPost, "/v1/auth/staff/login", api.StaffLoginRequest{Email: "gm@kaapiestate.test", Password: staffPassword}, nil)
	var cookies []string
	for _, c := range (&http.Response{Header: login.Header}).Cookies() {
		cookies = append(cookies, c.Name+"="+c.Value)
	}

	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"/v1/staff/changes/stream", nil)
	req.Header.Set("Cookie", strings.Join(cookies, "; "))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := bufio.NewScanner(res.Body)
	for lines.Scan() && lines.Text() != ": connected" {
	}

	// A hold changes availability: consoles hear "inventory".
	holdVilla(t, s.NewSession())
	for lines.Scan() {
		if lines.Text() == "data: inventory" {
			return
		}
	}
	t.Fatalf("no inventory change arrived: %v", lines.Err())
}

func TestChangeStreamNeedsAStaffPermission(t *testing.T) {
	s := apitest.New(t)
	guestLogin(t, s, "curious@example.test")
	if res := s.Do(http.MethodGet, "/v1/staff/changes/stream", nil, nil); res.Status != http.StatusForbidden {
		t.Fatalf("a guest must not get the stream: %d", res.Status)
	}
}

// The minute sweep runs UPDATEs that usually match nothing; those must not wake every console.
func TestOnlyRealChangesAreAnnounced(t *testing.T) {
	s := withChangeFeed(t)
	topics, stop := s.App.Changes.Subscribe()
	defer stop()
	drain := func(wait time.Duration) []string {
		var got []string
		deadline := time.After(wait)
		for {
			select {
			case topic := <-topics:
				got = append(got, topic)
			case <-deadline:
				return got
			}
		}
	}
	drain(500 * time.Millisecond) // the "*" sent when the feed connected

	for _, q := range []string{
		`UPDATE holds SET status = status WHERE false`,
		`UPDATE bookings SET updated_at = updated_at WHERE false`,
		`DELETE FROM promo_codes WHERE false`,
	} {
		if _, err := s.Pool.Exec(s.Ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if got := drain(time.Second); len(got) != 0 {
		t.Fatalf("statements that changed nothing were announced: %v", got)
	}

	holdVilla(t, s.NewSession())
	got := drain(2 * time.Second)
	for _, topic := range got {
		if topic == "inventory" {
			return
		}
	}
	t.Fatalf("a real hold must be announced as inventory, got %v", got)
}
