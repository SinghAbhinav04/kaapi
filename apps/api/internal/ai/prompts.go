package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kaapi/api/internal/domain"
)

// Prompts are shared by every provider so that switching models doesn't change the task.

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
If there are two intents, put the main one in category and the other in secondary_category (else "NONE").

Extract only what the message says. Never guess. Use null when absent.
- Dates: today is {{TODAY}} (Asia/Kolkata). Resolve "next weekend", "tonight", "14-16 Nov" against today; a month
  without a year is the next such month on or after today. check_out is the morning of departure. Format YYYY-MM-DD.
- adults/children/rooms for room stays; guest_count for events (total people).
- budget_inr as a number of rupees ("12 lakh" = 1200000, "80k" = 80000), budget_basis PER_NIGHT or TOTAL.
- event_month as YYYY-MM for events with a month but no exact dates.
confidence: your probability (0 to 1) that category is right. Use below 0.6 when the message is vague.
summary: one short English sentence a staff member can scan.`

// jsonShape is appended for providers that take JSON mode without a schema (Groq).
const jsonShape = `

Reply with one JSON object and nothing else, with exactly these keys:
category, secondary_category, confidence, language, summary, check_in, check_out, adults, children, rooms,
guest_count, budget_inr, budget_basis, event_month.`

const draftSystem = `You draft replies for the staff of Kaapi Estate, a coffee-estate resort in Chikkamagaluru.
A staff member will read and edit your draft before anything is sent.

Write a short, warm, specific reply (at most 120 words) in plain text, in English unless the guest wrote in Hindi script.
Hard rules:
- Never write any price, amount, rupee figure, discount or room count. Where prices or availability belong,
  write the placeholder {{QUOTE}} on its own line; the system replaces it with live figures.
- Start with "Hi {{GUEST_NAME}}," and end with {{SIGNATURE}} on its own line.
- For weddings and events, offer a call and include {{EVENT_TEAM_CONTACT}}.
- You write as STAFF, in the first person.
- For complaints: apologise sincerely, don't argue or admit legal fault. If STAFF is the General Manager, take
  personal ownership ("I'm looking into this myself"); otherwise say the General Manager will personally follow up.
- Only state resort facts that appear in the FACTS list. If asked something not covered, say the team will confirm.
- Never invent dates, policies or promises.`

func classifyPrompt(today time.Time) string {
	return strings.Replace(classifySystem, "{{TODAY}}", today.Format("Monday 2 January 2006"), 1)
}

func draftUserMessage(in DraftInput) string {
	var facts strings.Builder
	for _, f := range in.Facts {
		fmt.Fprintf(&facts, "- %s %s\n", f.Question, f.Answer)
	}
	return fmt.Sprintf("CATEGORY: %s\nTODAY: %s\nSTAFF: %s\nFACTS:\n%s\nGUEST MESSAGE:\n%s",
		in.Category, in.Today.Format("2 January 2006"), in.StaffName, facts.String(), in.Message)
}

// classifySchema is Gemini's response schema (OpenAPI subset).
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

type classifyOut struct {
	Category    string   `json:"category"`
	Secondary   *string  `json:"secondary_category"`
	Confidence  float64  `json:"confidence"`
	Language    string   `json:"language"`
	Summary     string   `json:"summary"`
	CheckIn     *string  `json:"check_in"`
	CheckOut    *string  `json:"check_out"`
	Adults      *float64 `json:"adults"`
	Children    *float64 `json:"children"`
	Rooms       *float64 `json:"rooms"`
	GuestCount  *float64 `json:"guest_count"`
	BudgetINR   *float64 `json:"budget_inr"`
	BudgetBasis *string  `json:"budget_basis"`
	EventMonth  *string  `json:"event_month"`
}

// decodeClassification turns any provider's JSON into a Classification, rejecting output
// that isn't the agreed shape. Values that are merely implausible are dropped later by Validate.
func decodeClassification(raw string) (Classification, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(raw, "```json"), "```"), "```")
	var out classifyOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return Classification{}, fmt.Errorf("not the agreed JSON: %w", err)
	}
	c := Classification{Category: domain.Category(strings.ToUpper(out.Category)), Confidence: out.Confidence,
		Language: out.Language, Summary: out.Summary}
	if !c.Category.Valid() {
		return Classification{}, fmt.Errorf("unknown category %q", out.Category)
	}
	if out.Secondary != nil {
		if s := domain.Category(strings.ToUpper(*out.Secondary)); s.Valid() && s != c.Category {
			c.Secondary = &s
		}
	}
	e := &c.Extracted
	e.CheckIn, e.CheckOut = parseDay(out.CheckIn), parseDay(out.CheckOut)
	e.Adults, e.Children, e.Rooms, e.GuestCount = toInt(out.Adults), toInt(out.Children), toInt(out.Rooms), toInt(out.GuestCount)
	e.EventMonth = out.EventMonth
	if out.BudgetINR != nil {
		b := int64(*out.BudgetINR)
		e.BudgetINR = &b
	}
	if out.BudgetBasis != nil {
		b := BudgetBasis(strings.ToUpper(*out.BudgetBasis))
		if b == BudgetPerNight || b == BudgetTotal {
			e.BudgetBasis = &b
		}
	}
	return c, nil
}

func toInt(f *float64) *int {
	if f == nil {
		return nil
	}
	n := int(*f)
	return &n
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
