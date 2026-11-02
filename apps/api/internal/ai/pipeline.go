package ai

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"kaapi/api/internal/domain"
)

// Result is the pipeline's verdict on one enquiry.
type Result struct {
	Classification
	Status       domain.AIStatus // DONE (model + rules) or FALLBACK (rules only)
	Model        string
	LatencyMS    int
	KeywordSaid  domain.Category
	ComplaintHit bool   // either classifier saw a complaint: always alert the GM
	Reason       string // why the confidence is what it is, shown next to the chip
}

// Classifier runs the model (when configured) and the keyword rules, validates the model's
// output and blends the two into one confidence.
type Classifier struct {
	provider Provider // nil: rules only
}

func NewClassifier(p Provider) *Classifier { return &Classifier{provider: p} }

func (c *Classifier) Enabled() bool { return c.provider != nil }

// Classify never fails: if the model is unavailable or returns nonsense, the keyword
// classifier's answer is used and the status says so. The caller retries FALLBACK later.
func (c *Classifier) Classify(ctx context.Context, text, senderName string, today time.Time) (Result, error) {
	runes := []rune(text)
	if len(runes) > maxMessageRunes {
		text = string(runes[:maxMessageRunes])
	}
	redacted := Redact(text, senderName)
	kw := KeywordClassify(text)

	if c.provider != nil {
		start := time.Now()
		m, err := c.provider.Classify(ctx, redacted, today)
		latency := int(time.Since(start).Milliseconds())
		if err == nil {
			Validate(&m, today)
			return blend(m, kw, c.provider.Name(), latency), nil
		}
		if errors.Is(err, context.Canceled) {
			return Result{}, err
		}
		r := fallback(kw)
		r.Model, r.LatencyMS, r.Reason = c.provider.Name(), latency, "AI unavailable, keyword rules only"
		return r, nil
	}
	r := fallback(kw)
	r.Reason = "AI not configured, keyword rules only"
	return r, nil
}

func fallback(kw Classification) Result {
	r := Result{Classification: kw, Status: domain.AIFallback, KeywordSaid: kw.Category}
	r.ComplaintHit = kw.Category == domain.CategoryComplaint || (kw.Secondary != nil && *kw.Secondary == domain.CategoryComplaint)
	r.Summary = ""
	return r
}

// blend adjusts the model's self-reported confidence, which is poorly calibrated on its own:
// agreement with the rules raises it, a confident disagreement lowers it, and missing
// details for a stay lower it a little.
func blend(m, kw Classification, model string, latency int) Result {
	r := Result{Classification: m, Status: domain.AIDone, Model: model, LatencyMS: latency, KeywordSaid: kw.Category}
	conf := clamp(m.Confidence)
	switch {
	case kw.Category == m.Category:
		conf += agreementBonus
		r.Reason = "AI and keyword rules agree"
	case kw.Confidence >= 0.6 && kw.Category != domain.CategoryOther:
		conf -= disagreementPenalty
		r.Reason = "AI and keyword rules disagree (rules said " + strings.ToLower(string(kw.Category)) + ")"
	default:
		r.Reason = "AI only; keyword rules had no strong view"
	}
	if m.Category == domain.CategoryRoom && (m.Extracted.CheckIn == nil || m.Extracted.CheckOut == nil) {
		conf -= missingFieldsPenalty
		r.Reason += "; dates missing"
	}
	r.Confidence = clamp(conf)
	r.ComplaintHit = m.Category == domain.CategoryComplaint || kw.Category == domain.CategoryComplaint ||
		(m.Secondary != nil && *m.Secondary == domain.CategoryComplaint)
	return r
}

// Validate drops extracted values that can't be right instead of trusting them.
func Validate(c *Classification, today time.Time) {
	e := &c.Extracted
	if e.CheckIn != nil && e.CheckIn.Before(today) {
		e.CheckIn = nil
	}
	if e.CheckIn != nil && e.CheckOut != nil {
		n := domain.NightsBetween(*e.CheckIn, *e.CheckOut)
		if n < 1 || n > maxNightsInMessage {
			e.CheckOut = nil
		}
	}
	if e.CheckIn == nil {
		e.CheckOut = nil
	}
	e.Adults = positive(e.Adults, maxGuests)
	e.Children = nonNegative(e.Children, maxGuests)
	e.Rooms = positive(e.Rooms, maxGuests)
	e.GuestCount = positive(e.GuestCount, maxGuests)
	if e.BudgetINR != nil && *e.BudgetINR <= 0 {
		e.BudgetINR, e.BudgetBasis = nil, nil
	}
	if e.EventMonth != nil {
		if _, err := time.Parse("2006-01", *e.EventMonth); err != nil {
			e.EventMonth = nil
		}
	}
	if !c.Category.Valid() {
		c.Category, c.Confidence = domain.CategoryOther, 0
	}
}

// ---- drafts

var (
	// Anything that looks like money in the model's own words: ₹, Rs, INR, rupees, lakh, "k".
	moneyRe       = regexp.MustCompile(`(?i)(₹\s?[\d,]+|\brs\.?\s?[\d,]+|\binr\s?[\d,]+|[\d,]+\s?(?:rupees|lakh|lakhs|lac|crore)\b|\b\d+\s?k\b)`)
	placeholderRe = regexp.MustCompile(`\{\{[A-Z_]+\}\}`)
	allowed       = map[string]bool{PlaceholderGuestName: true, PlaceholderQuote: true, PlaceholderEventTeam: true, PlaceholderSignature: true}
)

// Guard removes any money the model wrote itself (prices must come from the rate table)
// and any placeholder we don't fill. It reports why it changed the text, for the flag
// shown on the draft.
func Guard(draft string) (string, string) {
	var reasons []string
	if moneyRe.MatchString(draft) {
		var kept []string
		for _, line := range strings.Split(draft, "\n") {
			if moneyRe.MatchString(line) {
				continue
			}
			kept = append(kept, line)
		}
		draft = strings.Join(kept, "\n")
		reasons = append(reasons, "removed a line where the AI wrote a price itself")
	}
	draft = placeholderRe.ReplaceAllStringFunc(draft, func(p string) string {
		if allowed[p] {
			return p
		}
		reasons = append(reasons, "removed an unknown placeholder")
		return ""
	})
	return strings.TrimSpace(draft), strings.Join(reasons, "; ")
}

func clamp(v float64) float64 { return min(max(v, 0), 1) }

func positive(v *int, hi int) *int {
	if v == nil || *v < 1 || *v > hi {
		return nil
	}
	return v
}

func nonNegative(v *int, hi int) *int {
	if v == nil || *v < 0 || *v > hi {
		return nil
	}
	return v
}
