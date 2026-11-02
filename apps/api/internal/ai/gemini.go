package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"kaapi/api/internal/domain"
)

// Gemini calls the Gemini REST API directly: one dependency fewer than the SDK, and the
// request is plain to read. The key travels in a header, never in the URL, so it can't
// end up in access logs.
type Gemini struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client

	// Calls are spaced to stay under the plan's requests-per-minute limit (the free tier
	// allows about 10), so a burst of enquiries queues politely instead of failing with 429s.
	gap  time.Duration
	mu   sync.Mutex
	next time.Time
}

const (
	geminiBaseURL      = "https://generativelanguage.googleapis.com"
	geminiTimeout      = 20 * time.Second
	geminiKeyHeader    = "x-goog-api-key"
	maxGeminiRespBytes = 1 << 20
	draftTemperature   = 0.4
)

// ErrUnavailable means "try again later" (rate limit, outage, timeout): the worker retries
// and meanwhile uses the keyword classifier.
var ErrUnavailable = errors.New("ai: provider unavailable")

// NewGemini returns a client. rpm <= 0 means no pacing.
func NewGemini(apiKey, model, baseURL string, rpm int) *Gemini {
	if baseURL == "" {
		baseURL = geminiBaseURL
	}
	g := &Gemini{apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: geminiTimeout}}
	if rpm > 0 {
		g.gap = time.Minute / time.Duration(rpm)
	}
	return g
}

// wait blocks until this call's slot comes up.
func (g *Gemini) wait(ctx context.Context) error {
	if g.gap == 0 {
		return nil
	}
	g.mu.Lock()
	now := time.Now()
	slot := g.next
	if slot.Before(now) {
		slot = now
	}
	g.next = slot.Add(g.gap)
	g.mu.Unlock()
	if d := slot.Sub(now); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}

func (g *Gemini) Name() string { return "gemini:" + g.model }

// ---- classify

const classifySystem = `You read enquiries sent to Kaapi Estate, a coffee-estate resort in Chikkamagaluru, India.
Messages may be in English, Hindi, Hinglish (Hindi in Latin letters), Kannada-English or other languages.
Personal details have been replaced with [NAME], [EMAIL] and [PHONE].

Classify the message:
- ROOM: wants to stay, asks about rooms, availability, rates, changes to a room booking.
- WEDDING: weddings, engagements, receptions, pre-wedding shoots, family functions.
- OFFSITE: company or group events: offsites, conferences, retreats, team outings, reunions.
- COMPLAINT: unhappy about something that happened, even if politely phrased as a question
  (e.g. "is it normal that we were charged twice?"), refunds, threats to review or sue.
- OTHER: anything else: general questions, vendors, job applications, spam, unclear one-word messages.
If there are two intents, put the main one in category and the other in secondary_category.

Extract only what the message says. Never guess. Use null when absent.
- Dates: today is {{TODAY}} (Asia/Kolkata). Resolve "next weekend", "tonight", "14-16 Nov" against today; a month
  without a year is the next such month on or after today. check_out is the morning of departure.
- adults/children/rooms for room stays; guest_count for events (total people).
- budget_inr as a number of rupees ("12 lakh" = 1200000, "80k" = 80000), budget_basis PER_NIGHT or TOTAL.
- event_month as YYYY-MM for events with a month but no exact dates.
confidence: your probability (0 to 1) that category is right. Use below 0.6 when the message is vague.
summary: one short English sentence a staff member can scan.`

type classifyOut struct {
	Category    string   `json:"category"`
	Secondary   string   `json:"secondary_category"`
	Confidence  float64  `json:"confidence"`
	Language    string   `json:"language"`
	Summary     string   `json:"summary"`
	CheckIn     *string  `json:"check_in"`
	CheckOut    *string  `json:"check_out"`
	Adults      *int     `json:"adults"`
	Children    *int     `json:"children"`
	Rooms       *int     `json:"rooms"`
	GuestCount  *int     `json:"guest_count"`
	BudgetINR   *float64 `json:"budget_inr"`
	BudgetBasis *string  `json:"budget_basis"`
	EventMonth  *string  `json:"event_month"`
}

func classifySchema() map[string]any {
	cats := []string{"ROOM", "WEDDING", "OFFSITE", "COMPLAINT", "OTHER"}
	str := func(desc string) map[string]any {
		return map[string]any{"type": "STRING", "nullable": true, "description": desc}
	}
	num := func() map[string]any { return map[string]any{"type": "INTEGER", "nullable": true} }
	return map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"category":           map[string]any{"type": "STRING", "enum": cats},
			"secondary_category": map[string]any{"type": "STRING", "enum": append(cats, "NONE")},
			"confidence":         map[string]any{"type": "NUMBER"},
			"language":           map[string]any{"type": "STRING"},
			"summary":            map[string]any{"type": "STRING"},
			"check_in":           str("YYYY-MM-DD"),
			"check_out":          str("YYYY-MM-DD"),
			"adults":             num(),
			"children":           num(),
			"rooms":              num(),
			"guest_count":        num(),
			"budget_inr":         map[string]any{"type": "NUMBER", "nullable": true},
			"budget_basis":       map[string]any{"type": "STRING", "nullable": true, "enum": []string{"PER_NIGHT", "TOTAL"}},
			"event_month":        str("YYYY-MM"),
		},
		"required": []string{"category", "secondary_category", "confidence", "language", "summary"},
		"propertyOrdering": []string{"category", "secondary_category", "confidence", "language", "summary", "check_in",
			"check_out", "adults", "children", "rooms", "guest_count", "budget_inr", "budget_basis", "event_month"},
	}
}

func (g *Gemini) Classify(ctx context.Context, message string, today time.Time) (Classification, error) {
	system := strings.Replace(classifySystem, "{{TODAY}}", today.Format("Monday 2 January 2006"), 1)
	raw, err := g.generate(ctx, system, message, map[string]any{
		"temperature": 0, "responseMimeType": "application/json", "responseSchema": classifySchema(),
	})
	if err != nil {
		return Classification{}, err
	}
	var out classifyOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return Classification{}, fmt.Errorf("ai: model output is not the agreed JSON: %w", err)
	}
	c := Classification{Category: domain.Category(out.Category), Confidence: out.Confidence, Language: out.Language, Summary: out.Summary}
	if !c.Category.Valid() {
		return Classification{}, fmt.Errorf("ai: unknown category %q", out.Category)
	}
	if s := domain.Category(out.Secondary); s.Valid() && s != c.Category {
		c.Secondary = &s
	}
	e := &c.Extracted
	e.CheckIn, e.CheckOut = parseDay(out.CheckIn), parseDay(out.CheckOut)
	e.Adults, e.Children, e.Rooms, e.GuestCount, e.EventMonth = out.Adults, out.Children, out.Rooms, out.GuestCount, out.EventMonth
	if out.BudgetINR != nil {
		b := int64(*out.BudgetINR)
		e.BudgetINR = &b
	}
	if out.BudgetBasis != nil {
		b := BudgetBasis(*out.BudgetBasis)
		e.BudgetBasis = &b
	}
	return c, nil
}

// ---- draft

const draftSystem = `You draft replies for the staff of Kaapi Estate, a coffee-estate resort in Chikkamagaluru.
A staff member will read and edit your draft before anything is sent.

Write a short, warm, specific reply (at most 120 words) in plain text, in English unless the guest wrote in Hindi script.
Hard rules:
- Never write any price, amount, rupee figure, discount or room count. Where prices or availability belong,
  write the placeholder {{QUOTE}} on its own line; the system replaces it with live figures.
- Start with "Hi {{GUEST_NAME}}," and end with {{SIGNATURE}} on its own line.
- For weddings and events, offer a call and include {{EVENT_TEAM_CONTACT}}.
- For complaints: apologise sincerely, don't argue or admit legal fault, say the General Manager will personally follow up.
- Only state resort facts that appear in the FACTS list. If asked something not covered, say the team will confirm.
- Never invent dates, policies or promises.`

func (g *Gemini) Draft(ctx context.Context, in DraftInput) (string, error) {
	var facts strings.Builder
	for _, f := range in.Facts {
		fmt.Fprintf(&facts, "- %s %s\n", f.Question, f.Answer)
	}
	user := fmt.Sprintf("CATEGORY: %s\nTODAY: %s\nSTAFF: %s\nFACTS:\n%s\nGUEST MESSAGE:\n%s",
		in.Category, in.Today.Format("2 January 2006"), in.StaffName, facts.String(), in.Message)
	return g.generate(ctx, draftSystem, user, map[string]any{"temperature": draftTemperature})
}

// ---- transport

func (g *Gemini) generate(ctx context.Context, system, user string, config map[string]any) (string, error) {
	if err := g.wait(ctx); err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": system}}},
		"contents":          []map[string]any{{"role": "user", "parts": []map[string]string{{"text": user}}}},
		"generationConfig":  config,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, g.model), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(geminiKeyHeader, g.apiKey)
	res, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxGeminiRespBytes))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	switch {
	case res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500:
		return "", fmt.Errorf("%w: HTTP %d", ErrUnavailable, res.StatusCode)
	case res.StatusCode != http.StatusOK:
		return "", fmt.Errorf("ai: provider rejected the request: HTTP %d: %.200s", res.StatusCode, raw)
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ai: unreadable provider response: %w", err)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("ai: provider returned no answer (blocked or empty)")
	}
	var text strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}
	return strings.TrimSpace(text.String()), nil
}

func parseDay(s *string) *time.Time {
	if s == nil {
		return nil
	}
	d, err := time.Parse(time.DateOnly, strings.TrimSpace(*s))
	if err != nil {
		return nil
	}
	return &d
}
