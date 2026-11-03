package api_test

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"kaapi/api/internal/api"
	"kaapi/api/internal/gmail"
	"kaapi/api/internal/platform/config"
	"kaapi/api/tests/apitest"
	"kaapi/api/tests/testdb"
)

// fakeGoogle plays Google's OAuth and Gmail endpoints: one guest email waits in the inbox.
type fakeGoogle struct {
	mu       sync.Mutex
	sent     []map[string]string // decoded raw + threadId of each send
	failSend bool
	revoked  bool
}

func (f *fakeGoogle) handler(t *testing.T) http.Handler {
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		out := map[string]any{"access_token": "access-1", "expires_in": 3600}
		if r.Form.Get("grant_type") == "authorization_code" {
			if r.Form.Get("code") != "good-code" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			out["refresh_token"] = "refresh-1"
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("POST /revoke", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.revoked = true
		f.mu.Unlock()
	})
	mux.HandleFunc("GET /gmail/v1/users/me/profile", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"emailAddress": "stay@kaapiestate.test", "historyId": "100"})
	})
	mux.HandleFunc("GET /gmail/v1/users/me/history", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("startHistoryId") != "100" {
			_ = json.NewEncoder(w).Encode(map[string]any{"historyId": "101"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"historyId": "101", "history": []any{
			map[string]any{"messagesAdded": []any{map[string]any{"message": map[string]string{"id": "m1"}}}},
			map[string]any{"messagesAdded": []any{map[string]any{"message": map[string]string{"id": "own"}}}},
		}})
	})
	mux.HandleFunc("GET /gmail/v1/users/me/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		from, labels := "Asha Rao <asha@example.test>", []string{"INBOX", "UNREAD"}
		if r.PathValue("id") == "own" { // our own sent copy must never become an enquiry
			from, labels = "Kaapi <stay@kaapiestate.test>", []string{"SENT"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": r.PathValue("id"), "threadId": "t1", "labelIds": labels,
			"payload": map[string]any{"mimeType": "multipart/alternative",
				"headers": []map[string]string{{"name": "From", "value": from}, {"name": "Subject", "value": "Room for 14-16 Nov"},
					{"name": "Message-ID", "value": "<abc@mail.example.test>"}},
				"parts": []any{map[string]any{"mimeType": "text/plain", "body": map[string]string{
					"data": b64("Hi, do you have a room for two adults from 14 to 16 Nov?\n\nOn Mon, 2 Nov 2026 at 10:00, Kaapi <stay@kaapiestate.test> wrote:\n> old text")}}}}})
	})
	mux.HandleFunc("POST /gmail/v1/users/me/messages/send", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failSend {
			http.Error(w, "backend error", http.StatusInternalServerError)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, err := base64.RawURLEncoding.DecodeString(body["raw"])
		if err != nil {
			t.Errorf("send raw not base64url: %v", err)
		}
		f.sent = append(f.sent, map[string]string{"raw": string(raw), "threadId": body["threadId"]})
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "sent-" + string(rune('0'+len(f.sent))), "threadId": "t1"})
	})
	return mux
}

func TestGmailConnectImportReplyAndCompose(t *testing.T) {
	fake := &fakeGoogle{}
	google := httptest.NewServer(fake.handler(t))
	defer google.Close()

	cfg := apitest.TestConfig()
	cfg.Gmail = config.Gmail{ClientID: "client", ClientSecret: "secret", RedirectURL: "http://localhost:3000/api/v1/integrations/gmail/callback",
		PollEvery: time.Hour, OAuthBaseURL: google.URL, APIBaseURL: google.URL}
	s := apitest.NewWith(t, cfg)
	staffLogin(t, s, "admin@kaapiestate.test", nil)

	var st api.GmailStatusResponse
	s.Do(http.MethodGet, "/v1/staff/integrations/gmail", nil, nil).JSON(t, &st)
	if !st.Gmail.Configured || st.Gmail.Connected {
		t.Fatalf("before connecting: %+v", st.Gmail)
	}

	// Connect: the consent URL carries a signed state; Google sends the browser back with a code.
	var conn api.GmailConnectResponse
	s.Do(http.MethodPost, "/v1/staff/integrations/gmail/connect", nil, nil).JSON(t, &conn)
	u, err := url.Parse(conn.AuthURL)
	if err != nil || u.Query().Get("access_type") != "offline" || u.Query().Get("state") == "" {
		t.Fatalf("consent url %q", conn.AuthURL)
	}
	bad := s.NoRedirects().Do(http.MethodGet, "/v1/integrations/gmail/callback?code=good-code&state=forged.1.x", nil, nil)
	if !strings.Contains(bad.Header.Get("Location"), "gmail=error") {
		t.Fatalf("forged state: %d %s", bad.Status, bad.Header.Get("Location"))
	}
	res := s.NoRedirects().Do(http.MethodGet, "/v1/integrations/gmail/callback?code=good-code&state="+url.QueryEscape(u.Query().Get("state")), nil, nil)
	if res.Status != http.StatusFound || !strings.Contains(res.Header.Get("Location"), "gmail=connected") {
		t.Fatalf("callback: %d %s", res.Status, res.Header.Get("Location"))
	}
	var sealed []byte
	if err := s.Pool.QueryRow(s.Ctx, `SELECT refresh_token_enc FROM integrations WHERE provider = 'GMAIL'`).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), "refresh-1") {
		t.Fatal("refresh token stored in the clear")
	}

	// Import: the guest's email becomes one enquiry; our own sent copy is ignored; a repeat poll adds nothing.
	for range 2 {
		if r := s.Do(http.MethodPost, "/v1/staff/integrations/gmail/sync", nil, nil); r.Status != http.StatusOK {
			t.Fatalf("sync: %d %s", r.Status, r.Body)
		}
	}
	var ref, text string
	var n int
	if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) OVER (), ref, raw_text FROM enquiries WHERE external_id IS NOT NULL`).Scan(&n, &ref, &text); err != nil {
		t.Fatal(err)
	}
	if n != 1 || strings.Contains(text, "old text") || !strings.Contains(text, "two adults") {
		t.Fatalf("imported %d enquiries, text %q", n, text)
	}

	// Reply: a staff-written draft goes out through Gmail, in the guest's thread.
	if r := s.Do(http.MethodPut, "/v1/staff/enquiries/"+ref+"/draft", api.EditDraftRequest{Body: "Dear Asha, yes we do."}, nil); r.Status != http.StatusOK {
		t.Fatalf("draft: %d %s", r.Status, r.Body)
	}
	fake.failSend = true
	if r := s.Do(http.MethodPost, "/v1/staff/enquiries/"+ref+"/send", nil, nil); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("send while Gmail is down: %d, want 503", r.Status)
	}
	fake.failSend = false // the failed attempt must leave the draft sendable
	if r := s.Do(http.MethodPost, "/v1/staff/enquiries/"+ref+"/send", nil, nil); r.Status != http.StatusOK {
		t.Fatalf("send: %d %s", r.Status, r.Body)
	}
	if r := s.Do(http.MethodPost, "/v1/staff/enquiries/"+ref+"/send", nil, nil); r.Status != http.StatusConflict {
		t.Fatalf("second send: %d, want 409", r.Status)
	}
	if len(fake.sent) != 1 {
		t.Fatalf("gmail got %d sends, want 1", len(fake.sent))
	}
	reply := fake.sent[0]
	if reply["threadId"] != "t1" || !strings.Contains(reply["raw"], "In-Reply-To: <abc@mail.example.test>") ||
		!strings.Contains(reply["raw"], "To: asha@example.test") || !strings.Contains(reply["raw"], "Subject: Re: Room for 14-16 Nov") {
		t.Fatalf("reply not threaded:\n%s", reply["raw"])
	}
	var status string
	if err := s.Pool.QueryRow(s.Ctx, `SELECT status FROM outbox_messages WHERE kind = 'ENQUIRY_REPLY' ORDER BY created_at DESC LIMIT 1`).Scan(&status); err != nil || status != "SENT" {
		t.Fatalf("outbox status %q err %v", status, err)
	}

	// Compose: a new message from scratch.
	var composed api.OutboxMessageResponse
	r := s.Do(http.MethodPost, "/v1/staff/messages", api.ComposeRequest{To: "planner@example.test", Subject: "Your wedding dates", Body: "Hello"}, nil)
	r.JSON(t, &composed)
	if r.Status != http.StatusCreated || composed.Message.Status != "SENT" || composed.Message.Kind != "STAFF_MESSAGE" || len(fake.sent) != 2 {
		t.Fatalf("compose: %d %+v sends=%d", r.Status, composed.Message, len(fake.sent))
	}
	if r := s.Do(http.MethodPost, "/v1/staff/messages", api.ComposeRequest{To: "not an email", Subject: "x", Body: "y"}, nil); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("bad recipient: %d", r.Status)
	}

	// Disconnect: Google's grant is revoked and replies go back to being simulated.
	s.Do(http.MethodDelete, "/v1/staff/integrations/gmail", nil, nil).JSON(t, &st)
	if st.Gmail.Connected || !fake.revoked {
		t.Fatalf("after disconnect: %+v revoked=%v", st.Gmail, fake.revoked)
	}

	// Reservations staff can see the status but not connect a mailbox.
	other := s.NewSession()
	staffLogin(t, other, "reservations@kaapiestate.test", nil)
	if r := other.Do(http.MethodPost, "/v1/staff/integrations/gmail/connect", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("reservations connect: %d, want 403", r.Status)
	}
}

// Without a mailbox, sending stays simulated exactly as before.
func TestComposeWithoutGmailIsSimulated(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "sales@kaapiestate.test", nil)
	var out api.OutboxMessageResponse
	r := s.Do(http.MethodPost, "/v1/staff/messages", api.ComposeRequest{To: "planner@example.test", Subject: "Dates", Body: "Hello"}, nil)
	r.JSON(t, &out)
	if r.Status != http.StatusCreated || out.Message.Status != "SIMULATED" {
		t.Fatalf("%d %+v", r.Status, out.Message)
	}
	if r := s.Do(http.MethodPost, "/v1/staff/integrations/gmail/connect", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("sales connect: %d, want 403", r.Status)
	}
}

// With GMAIL_REFRESH_TOKEN set, the server connects itself on start: no button, and a
// re-seed or fresh database comes back connected.
func TestGmailBootstrapFromEnvironment(t *testing.T) {
	fake := &fakeGoogle{}
	google := httptest.NewServer(fake.handler(t))
	defer google.Close()
	ctx, pool := testdb.Seeded(t)
	cfg := config.Gmail{ClientID: "client", ClientSecret: "secret", RedirectURL: "http://localhost:3000", PollEvery: time.Hour,
		RefreshToken: "refresh-1", OAuthBaseURL: google.URL, APIBaseURL: google.URL}
	svc := gmail.NewService(pool, cfg, apitest.TestConfig().JWESecret, time.Now, slog.New(slog.DiscardHandler))

	for range 2 { // the second start finds the same grant and leaves the row alone
		if err := svc.Bootstrap(ctx); err != nil {
			t.Fatal(err)
		}
	}
	st, err := svc.Status(ctx)
	if err != nil || !st.Connected || st.AccountEmail != "stay@kaapiestate.test" {
		t.Fatalf("status %+v err %v", st, err)
	}
	if token, _, err := svc.RefreshToken(ctx); err != nil || token != "refresh-1" {
		t.Fatalf("stored token %q err %v", token, err)
	}
}
