package ai_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
)

// stub is a model that fails with err (if set) and counts its calls.
type stub struct {
	name  string
	err   error
	calls atomic.Int32
}

func (s *stub) Name() string { return s.name }
func (s *stub) Classify(context.Context, string, time.Time) (ai.Classification, error) {
	s.calls.Add(1)
	if s.err != nil {
		return ai.Classification{}, s.err
	}
	return ai.Classification{Category: domain.CategoryRoom, Confidence: 0.9, AnsweredBy: s.name}, nil
}
func (s *stub) Draft(context.Context, ai.DraftInput) (string, error) {
	s.calls.Add(1)
	return "Hi {{GUEST_NAME}},\n{{SIGNATURE}}", s.err
}

func failing(name string, kind ai.FailKind) *stub {
	return &stub{name: name, err: &ai.ProviderError{Model: name, Kind: kind, Err: errors.New("boom")}}
}

func answeredBy(t *testing.T, c *ai.Chain) string {
	t.Helper()
	got, err := c.Classify(context.Background(), "villa?", today)
	if err != nil {
		t.Fatal(err)
	}
	return got.AnsweredBy
}

func TestChainRoundRobinsWithinATier(t *testing.T) {
	a, b, c := &stub{name: "a"}, &stub{name: "b"}, &stub{name: "c"}
	chain := ai.NewChain(ai.Tier{Providers: []ai.Provider{a, b, c}})
	var order []string
	for range 6 {
		order = append(order, answeredBy(t, chain))
	}
	if want := []string{"a", "b", "c", "a", "b", "c"}; !equal(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
}

func TestRateLimitedModelRestsAndTheNextAnswers(t *testing.T) {
	a, b := failing("a", ai.FailRateLimited), &stub{name: "b"}
	chain := ai.NewChain(ai.Tier{Providers: []ai.Provider{a, b}})
	for range 4 {
		if got := answeredBy(t, chain); got != "b" {
			t.Fatalf("answered by %s", got)
		}
	}
	if a.calls.Load() != 1 {
		t.Fatalf("a resting model must not be retried at once: %d calls", a.calls.Load())
	}
}

func TestBadOutputMovesOnWithoutResting(t *testing.T) {
	a, b := failing("a", ai.FailBadOutput), &stub{name: "b"}
	chain := ai.NewChain(ai.Tier{Providers: []ai.Provider{a, b}})
	answeredBy(t, chain)
	answeredBy(t, chain)
	answeredBy(t, chain)
	if a.calls.Load() < 2 {
		t.Fatalf("one odd answer shouldn't bench a model: %d calls", a.calls.Load())
	}
}

func TestGroqTakesOverWhenEveryGeminiModelFails(t *testing.T) {
	g1, g2 := failing("gemini:a", ai.FailUnavailable), failing("gemini:b", ai.FailRejected)
	pro := failing("gemini:pro", ai.FailRateLimited)
	groq := &stub{name: "groq:x"}
	chain := ai.NewChain(
		ai.Tier{Providers: []ai.Provider{g1, g2}},
		ai.Tier{Providers: []ai.Provider{pro}},
		ai.Tier{Providers: []ai.Provider{groq}},
	)
	if got := answeredBy(t, chain); got != "groq:x" {
		t.Fatal(got)
	}
	if g1.calls.Load() != 1 || g2.calls.Load() != 1 || pro.calls.Load() != 1 {
		t.Fatal("each Gemini model should be tried once before Groq")
	}
	if got, err := chain.Draft(context.Background(), ai.DraftInput{}); err != nil || got == "" {
		t.Fatalf("drafts use the same chain: %q %v", got, err)
	}
}

func TestEveryModelDownIsUnavailable(t *testing.T) {
	chain := ai.NewChain(ai.Tier{Providers: []ai.Provider{failing("a", ai.FailRateLimited), failing("b", ai.FailUnavailable)}})
	if _, err := chain.Classify(context.Background(), "hi", today); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("%v", err)
	}
	// Both are resting now; the next call fails fast instead of hammering them.
	start := time.Now()
	if _, err := chain.Classify(context.Background(), "hi", today); !errors.Is(err, ai.ErrUnavailable) || time.Since(start) > time.Second {
		t.Fatalf("%v after %s", err, time.Since(start))
	}
}

func TestGeminiRateLimitHonoursRetryDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[
			{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"37s"}]}}`))
	}))
	defer srv.Close()
	_, err := ai.NewGemini("k", "gemini-x", srv.URL).Classify(context.Background(), "hi", today)
	var pe *ai.ProviderError
	if !errors.As(err, &pe) || pe.Kind != ai.FailRateLimited || pe.RetryAfter != 37*time.Second || !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("%#v", err)
	}
}

func TestGroqClassifiesThroughJSONMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"category\":\"wedding\",\"secondary_category\":null,\"confidence\":0.9,\"language\":\"hinglish\",\"summary\":\"Wedding\",\"guest_count\":300,\"event_month\":\"2027-04\",\"adults\":null}"}}]}`))
	}))
	defer srv.Close()
	c, err := ai.NewGroq("k", "openai/gpt-oss-120b", srv.URL).Classify(context.Background(), "shaadi", today)
	if err != nil {
		t.Fatal(err)
	}
	if c.Category != domain.CategoryWedding || c.Extracted.GuestCount == nil || *c.Extracted.GuestCount != 300 || c.AnsweredBy != "groq:openai/gpt-oss-120b" {
		t.Fatalf("%+v", c)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
