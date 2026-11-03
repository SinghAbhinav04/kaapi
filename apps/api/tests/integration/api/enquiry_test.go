package api_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/config"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

// fakeGemini stands in for the Gemini API. It records every request body so tests can
// prove personal data never leaves, and can be switched to fail like an outage.
type fakeGemini struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
	down   atomic.Bool
	calls  atomic.Int32
}

func newFakeGemini(t *testing.T) *fakeGemini {
	f := &fakeGemini{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.Header.Get("x-goog-api-key") != "test-key" || strings.Contains(r.URL.RawQuery, "key=") {
			http.Error(w, "key must travel in the header", http.StatusUnauthorized)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.bodies = append(f.bodies, string(raw))
		f.mu.Unlock()
		if f.down.Load() { // the free tier's usual failure: out of quota, retry in a second
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":429,"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"1s"}]}}`))
			return
		}
		var req struct {
			Contents []struct {
				Parts []struct{ Text string } `json:"parts"`
			} `json:"contents"`
			GenerationConfig map[string]any `json:"generationConfig"`
		}
		_ = json.Unmarshal(raw, &req)
		msg := req.Contents[0].Parts[0].Text
		var text string
		if _, structured := req.GenerationConfig["responseSchema"]; structured {
			text = fakeClassify(msg)
		} else {
			// A draft that breaks the rules: it states a price itself.
			text = "Hi {{GUEST_NAME}},\n\nOur villas start at ₹15,000 a night.\n\n{{QUOTE}}\n\nWarm regards,\n{{SIGNATURE}}"
		}
		out, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{map[string]any{"text": text}}}, "finishReason": "STOP"}}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}))
	t.Cleanup(f.Close)
	return f
}

func fakeClassify(msg string) string {
	switch {
	case strings.Contains(msg, "villa free 14-16 Nov"):
		return `{"category":"ROOM","secondary_category":"NONE","confidence":0.93,"language":"en","summary":"Villa for 2 adults and a child, 14-16 Nov.",
			"check_in":"2026-11-14","check_out":"2026-11-16","adults":2,"children":1,"rooms":null,"guest_count":null,"budget_inr":null,"budget_basis":null,"event_month":null}`
	case strings.Contains(msg, "charged twice"):
		// The model misses the disguised complaint; the keyword rules must still alert the GM.
		return `{"category":"OTHER","secondary_category":"NONE","confidence":0.7,"language":"en","summary":"Question about a dinner bill."}`
	default:
		return `{"category":"OTHER","secondary_category":"NONE","confidence":0.3,"language":"en","summary":"Unclear."}`
	}
}

func (f *fakeGemini) sawText(s string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.bodies {
		if strings.Contains(b, s) {
			return true
		}
	}
	return false
}

func withGemini(t *testing.T) (*apitest.Server, *fakeGemini) {
	f := newFakeGemini(t)
	cfg := apitest.TestConfig()
	cfg.AI = config.AI{GeminiKey: "test-key", GeminiModels: []string{"gemini-test"}, GeminiBaseURL: f.URL}
	return apitest.NewWith(t, cfg), f
}

func process(t *testing.T, s *apitest.Server) int {
	t.Helper()
	n, err := s.App.ProcessEnquiries(s.Ctx)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func getEnquiry(t *testing.T, s *apitest.Server, ref string) api.EnquiryDTO {
	t.Helper()
	res := s.Do(http.MethodGet, "/v1/staff/enquiries/"+ref, nil, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("get %s: %d %s", ref, res.Status, res.Body)
	}
	var out api.EnquiryResponse
	res.JSON(t, &out)
	return out.Enquiry
}

func listLane(t *testing.T, s *apitest.Server, query string) []api.EnquirySummaryDTO {
	t.Helper()
	res := s.Do(http.MethodGet, "/v1/staff/enquiries?"+query, nil, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("list: %d %s", res.Status, res.Body)
	}
	var out api.EnquiriesResponse
	res.JSON(t, &out)
	return out.Enquiries
}

// The brief's ENQ-4411: the model reads it, routing sends it to Reservations, and the
// draft's prices come from the rate table, not from the model.
func TestModelDraftGetsLivePricesAndNeverItsOwn(t *testing.T) {
	s, gem := withGemini(t)
	if n := process(t, s); n != 60 {
		t.Fatalf("processed %d of 60 seeded enquiries", n)
	}
	staffLogin(t, s, "reservations@kaapiestate.test", nil)
	e := getEnquiry(t, s, "ENQ-4411")

	if e.Category == nil || *e.Category != domain.CategoryRoom || e.Lane != domain.LaneReservations || e.AIStatus != domain.AIDone {
		t.Fatalf("%+v", e)
	}
	if e.Extracted.CheckIn == nil || *e.Extracted.CheckIn != "2026-11-14" || e.Extracted.Nights == nil || *e.Extracted.Nights != 2 {
		t.Fatalf("extracted %+v", e.Extracted)
	}
	d := e.Draft
	if d == nil || !d.QuoteFilled || d.FlaggedReason == nil {
		t.Fatalf("draft %+v", d)
	}
	// Coffee Villa 14–16 Nov: ₹20,100 + ₹17,500 room only, 18% GST = ₹44,368; with breakfast ₹47,200.
	for _, want := range []string{"Hi Arjun,", "Coffee Villa: ₹44,368 room only, ₹47,200 with breakfast (1 left)", "Rohan"} {
		if !strings.Contains(d.Body, want) {
			t.Errorf("draft lacks %q:\n%s", want, d.Body)
		}
	}
	if strings.Contains(d.Body, "15,000") || strings.Contains(d.Body, "{{") {
		t.Errorf("model's own price or a placeholder survived:\n%s", d.Body)
	}
	// Redaction: the model never saw the guest's name or number.
	if gem.sawText("Arjun") || gem.sawText("90000 10002") || gem.sawText("example.test") {
		t.Error("personal data reached the model")
	}
}

func TestComplaintSeenByEitherClassifierAlertsTheGM(t *testing.T) {
	s, _ := withGemini(t)
	process(t, s)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	e := getEnquiry(t, s, "ENQ-4420") // "is it normal that we were charged twice…"
	if e.Lane != domain.LaneGM || e.Priority != domain.PriorityHigh {
		t.Fatalf("lane %s priority %s", e.Lane, e.Priority)
	}
	var alerts int
	if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) FROM notifications n JOIN enquiries e ON e.id = n.enquiry_id
		WHERE e.ref = 'ENQ-4420' AND n.type = 'COMPLAINT_ALERT' AND n.recipient_role = 'GM'`).Scan(&alerts); err != nil || alerts != 1 {
		t.Fatalf("alerts %d %v", alerts, err)
	}
	if len(outboxTo(t, s, domain.OutboxAlert, "gm@kaapiestate.test")) == 0 {
		t.Fatal("GM got no alert email")
	}
	// The GM's queue puts unanswered complaints first.
	if list := listLane(t, s, "lane=GM"); len(list) == 0 || list[0].Priority != domain.PriorityHigh {
		t.Fatalf("complaints should lead: %+v", list)
	}
}

func TestModelOutageFallsBackAndRetries(t *testing.T) {
	s, gem := withGemini(t)
	gem.down.Store(true)
	process(t, s)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	e := getEnquiry(t, s, "ENQ-4411")
	if e.AIStatus != domain.AIFallback || e.Category == nil || *e.Category != domain.CategoryRoom || !strings.Contains(e.AIReason, "unavailable") {
		t.Fatalf("%+v", e)
	}
	if e.Draft == nil || e.Draft.FlaggedReason == nil {
		t.Fatal("a template draft is still written, and says why")
	}

	gem.down.Store(false)
	time.Sleep(1100 * time.Millisecond) // the model rests for the retryDelay it asked for
	if res := s.Do(http.MethodPost, "/v1/staff/enquiries/enq-4411/retry-ai", nil, nil); res.Status != http.StatusOK {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	process(t, s)
	if e := getEnquiry(t, s, "ENQ-4411"); e.AIStatus != domain.AIDone {
		t.Fatalf("after retry: %s", e.AIStatus)
	}
}

func TestLanesAreScopedByRole(t *testing.T) {
	s := apitest.New(t) // no model: keyword rules only
	process(t, s)

	res := staffLogin(t, s, "reservations@kaapiestate.test", nil)
	_ = res
	for _, e := range listLane(t, s, "") {
		if e.Lane != domain.LaneReservations {
			t.Fatalf("reservations saw %s in %s", e.Ref, e.Lane)
		}
	}
	if r := s.Do(http.MethodGet, "/v1/staff/enquiries?lane=GM", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("%d", r.Status)
	}
	if r := s.Do(http.MethodGet, "/v1/staff/enquiries/ENQ-4420", nil, nil); r.Status != http.StatusNotFound {
		t.Fatalf("another lane's enquiry must not exist for this role: %d", r.Status)
	}

	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	lanes := map[domain.Lane]int{}
	for _, e := range listLane(t, gm, "limit=300&status=NEW") {
		lanes[e.Lane]++
		if e.AIStatus != domain.AIFallback {
			t.Fatalf("%s: %s", e.Ref, e.AIStatus)
		}
	}
	if lanes[domain.LaneUnsorted] == 0 || lanes[domain.LaneGM] == 0 || lanes[domain.LaneSales] == 0 {
		t.Fatalf("GM should see every lane: %v", lanes)
	}
	// "price?" is too vague to route: it waits in Unsorted for the GM.
	if e := getEnquiry(t, gm, "ENQ-4417"); e.Lane != domain.LaneUnsorted {
		t.Fatalf("ENQ-4417 in %s", e.Lane)
	}
}

func TestRerouteIsOneClickAndLogged(t *testing.T) {
	s := apitest.New(t)
	process(t, s)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	before := getEnquiry(t, s, "ENQ-4417")

	res := s.Do(http.MethodPost, "/v1/staff/enquiries/ENQ-4417/route", api.RouteEnquiryRequest{Lane: "RESERVATIONS", Category: "ROOM"}, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	var out api.EnquiryResponse
	res.JSON(t, &out)
	e := out.Enquiry
	if e.Lane != domain.LaneReservations || *e.Category != domain.CategoryRoom || e.AssignedTo == nil || e.AssignedTo.Name != "Rohan Shetty" {
		t.Fatalf("%+v", e)
	}
	if e.Draft == nil || e.Draft.Version != before.Draft.Version+1 {
		t.Fatal("a new category gets a fresh draft")
	}
	seen := map[domain.EnquiryEventType]bool{}
	for _, ev := range e.Events {
		seen[ev.Type] = true
	}
	if !seen[domain.EventRerouted] || !seen[domain.EventCategoryChanged] {
		t.Fatalf("events %+v", e.Events)
	}

	sales := s.NewSession()
	staffLogin(t, sales, "sales@kaapiestate.test", nil)
	if r := sales.Do(http.MethodGet, "/v1/staff/enquiries/ENQ-4417", nil, nil); r.Status != http.StatusNotFound {
		t.Fatalf("sales sees a reservations enquiry: %d", r.Status)
	}
	if r := s.Do(http.MethodPost, "/v1/staff/enquiries/ENQ-4417/route", api.RouteEnquiryRequest{Lane: "NOWHERE"}, nil); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("%d", r.Status)
	}

	var dash api.DashboardResponse
	s.Do(http.MethodGet, "/v1/staff/dashboard", nil, nil).JSON(t, &dash)
	if dash.Overbookings != 0 || dash.AIEnabled || dash.AICorrectionRate == nil || *dash.AICorrectionRate <= 0 {
		t.Fatalf("%+v", dash)
	}
}

// A guest asks from the website; staff fix the dates, edit, and send. Only Send reaches the guest.
func TestEnquiryToReply(t *testing.T) {
	s := apitest.New(t)
	ask := api.CreateEnquiryRequest{Name: "Kavya Rao", Email: "kavya@example.test",
		Message: "Is a villa free for our anniversary? 2 adults."}
	res := s.Do(http.MethodPost, "/v1/enquiries", ask, nil)
	if res.Status != http.StatusCreated {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	var created api.EnquiryCreatedResponse
	res.JSON(t, &created)
	var again api.EnquiryCreatedResponse
	s.Do(http.MethodPost, "/v1/enquiries", api.CreateEnquiryRequest{Name: "Kavya Rao", Email: "KAVYA@example.test",
		Message: "is a villa free for our anniversary  2 adults"}, nil).JSON(t, &again)
	if again.Ref != created.Ref {
		t.Fatalf("a repeat should not create a second enquiry: %s vs %s", again.Ref, created.Ref)
	}
	process(t, s)

	staffLogin(t, s, "reservations@kaapiestate.test", nil)
	e := getEnquiry(t, s, created.Ref)
	if e.Lane != domain.LaneReservations || e.Draft == nil || e.Draft.QuoteFilled {
		t.Fatalf("no dates yet, so no quote: %+v", e)
	}

	res = s.Do(http.MethodPatch, "/v1/staff/enquiries/"+created.Ref+"/fields",
		api.CorrectEnquiryRequest{CheckIn: ptr("2026-11-14"), CheckOut: ptr("2026-11-16"), Adults: ptr(2)}, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	var fixed api.EnquiryResponse
	res.JSON(t, &fixed)
	if !fixed.Enquiry.Draft.QuoteFilled || !strings.Contains(fixed.Enquiry.Draft.Body, "₹44,368") {
		t.Fatalf("corrected dates should re-price the draft:\n%s", fixed.Enquiry.Draft.Body)
	}

	edited := fixed.Enquiry.Draft.Body + "\n\nP.S. Happy anniversary!"
	if r := s.Do(http.MethodPut, "/v1/staff/enquiries/"+created.Ref+"/draft", api.EditDraftRequest{Body: edited}, nil); r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if outboxBefore := len(outboxTo(t, s, domain.OutboxEnquiryReply, "kavya@example.test")); outboxBefore != 0 {
		t.Fatal("nothing is sent until a person presses Send")
	}

	res = s.Do(http.MethodPost, "/v1/staff/enquiries/"+created.Ref+"/send", nil, nil)
	var sent api.EnquiryResponse
	res.JSON(t, &sent)
	if res.Status != http.StatusOK || sent.Enquiry.Status != domain.EnquiryReplied || sent.Enquiry.FirstReplyAt == nil {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	replies := outboxTo(t, s, domain.OutboxEnquiryReply, "kavya@example.test")
	if len(replies) != 1 {
		t.Fatalf("reply copies: %d", len(replies))
	}
	if body := replies[0].Body; body != edited {
		t.Fatalf("sent text differs from the staff's edit:\n%s", body)
	}
	if r := s.Do(http.MethodPost, "/v1/staff/enquiries/"+created.Ref+"/send", nil, nil); r.Status != http.StatusConflict {
		t.Fatalf("double send: %d", r.Status)
	}
}

func TestEnquiryIntakeValidation(t *testing.T) {
	s := apitest.New(t)
	if r := s.Do(http.MethodPost, "/v1/enquiries", api.CreateEnquiryRequest{Name: "A", Message: "hello"}, nil); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("no contact: %d", r.Status)
	}
	if r := s.Do(http.MethodPost, "/v1/enquiries", api.CreateEnquiryRequest{Email: "a@example.test"}, nil); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("no message: %d", r.Status)
	}
	var last int
	for i := 0; i < 6; i++ {
		last = s.Do(http.MethodPost, "/v1/enquiries", api.CreateEnquiryRequest{Email: "flood@example.test",
			Message: "message number " + strings.Repeat("x", i+1)}, nil).Status
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("sixth message in an hour: %d", last)
	}

	// The event form becomes a message the classifier can read.
	res := s.Do(http.MethodPost, "/v1/enquiries", api.CreateEnquiryRequest{Name: "Neha", Phone: "+91 90000 22222", Kind: "EVENT",
		Event: &api.EventDetailsDTO{Type: "Wedding", Month: "2027-02", GuestCount: 180, Nights: 2}}, nil)
	var c api.EnquiryCreatedResponse
	res.JSON(t, &c)
	process(t, s)
	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	if e := getEnquiry(t, gm, c.Ref); e.Channel != domain.ChannelEventForm || e.Lane != domain.LaneSales || !strings.Contains(e.Draft.Body, "Karthik") {
		t.Fatalf("%+v", e)
	}
}

// The inbox updates live: a new enquiry reaches an open stream as an event carrying its ref.
func TestInboxStreamPushesChanges(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	res := s.Stream("/v1/staff/inbox/stream")
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	waitFor := func(prefix string) string {
		deadline := time.After(5 * time.Second)
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatalf("stream closed waiting for %q", prefix)
				}
				if strings.HasPrefix(l, prefix) {
					return l
				}
			case <-deadline:
				t.Fatalf("no %q within 5s", prefix)
			}
		}
	}
	waitFor(": connected")

	var created struct {
		Ref string `json:"ref"`
	}
	r := s.Do(http.MethodPost, "/v1/staff/inbound", map[string]any{"channel": "EMAIL", "sender_email": "live@example.test", "text": "Do you have a room this weekend?"}, nil)
	r.JSON(t, &created)
	if created.Ref == "" {
		t.Fatalf("inbound: %d %s", r.Status, r.Body)
	}
	waitFor("event: inbox")
	if got := waitFor("data: "); got != "data: "+created.Ref {
		t.Fatalf("event data %q, want %q", got, created.Ref)
	}
}
