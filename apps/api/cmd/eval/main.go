// Command eval measures the enquiry classifier on the 60 labelled enquiries in
// seed/data/enquiries.json and writes docs/ai-eval-report.md.
//
//	go run ./cmd/eval                          # keyword rules only
//	GEMINI_API_KEY=… go run ./cmd/eval         # Gemini + rules, compared with rules alone
//
// It reports what matters for this resort: how often a message is auto-routed to the
// wrong team (costly), how often it waits in Unsorted (safe but slower), and whether every
// complaint reaches the GM.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/seed"
)

// thresholds are the Unsorted cut-offs the sweep compares.
var thresholds = []float64{0.4, 0.5, 0.55, 0.6, 0.65, 0.7, 0.8}

// extractedFields are the labels compared for extraction accuracy.
var extractedFields = []string{"check_in", "check_out", "adults", "children", "guest_count", "event_month"}

type outcome struct {
	fx  seed.EnquiryFx
	res ai.Result
}

type run struct {
	name     string
	outcomes []outcome
}

func main() {
	fixtures := flag.String("fixtures", "seed/data", "fixture directory")
	out := flag.String("out", "../../docs/ai-eval-report.md", "report path")
	flag.Parse()

	fx, err := seed.Load(*fixtures)
	if err != nil {
		log.Fatal(err)
	}
	today, err := referenceDay(filepath.Join(*fixtures, "enquiries.json"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	runs := []run{classify(ctx, "Keyword rules only", ai.NewClassifier(nil), fx.Enquiries, today)}
	model := ""
	if key := os.Getenv(config.EnvGeminiAPIKey); key != "" {
		model = envOr(config.EnvGeminiModel, "gemini-2.5-flash")
		g := ai.NewGemini(key, model, os.Getenv(config.EnvGeminiBaseURL), 10)
		runs = append(runs, classify(ctx, "Gemini ("+model+") + keyword rules", ai.NewClassifier(g), fx.Enquiries, today))
	}

	report := render(runs, today, model)
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s\n", *out)
	for _, r := range runs {
		m := measure(r, ai.UnsortedBelow)
		fmt.Printf("%-40s category %2d/%d  wrong team %d  unsorted %d  complaints caught %d/%d\n",
			r.name, m.correct, m.total, m.misrouted, m.unsorted, m.complaintsCaught, m.complaints)
	}
}

func classify(ctx context.Context, name string, c *ai.Classifier, fxs []seed.EnquiryFx, today time.Time) run {
	r := run{name: name}
	for i, e := range fxs {
		res, err := c.Classify(ctx, e.Text, e.Sender.Name, today)
		if err != nil {
			log.Fatalf("%s: %v", e.Ref, err)
		}
		if c.Enabled() && res.Status != domain.AIDone {
			log.Printf("%s: model unavailable, rules used (%s)", e.Ref, res.Reason)
		}
		r.outcomes = append(r.outcomes, outcome{fx: e, res: res})
		if c.Enabled() {
			fmt.Fprintf(os.Stderr, "\r%s: %d/%d", name, i+1, len(fxs))
		}
	}
	if c.Enabled() {
		fmt.Fprintln(os.Stderr)
	}
	return r
}

type metrics struct {
	total, correct                int
	misrouted, unsorted, routedOK int
	complaints, complaintsCaught  int
	falseAlerts                   int
	confusion                     map[domain.Category]map[domain.Category]int
	fieldsRight, fieldsTotal      map[string]int
	misses                        []outcome
}

func measure(r run, threshold float64) metrics {
	m := metrics{confusion: map[domain.Category]map[domain.Category]int{}, fieldsRight: map[string]int{}, fieldsTotal: map[string]int{}}
	for _, o := range r.outcomes {
		want := domain.Category(fmt.Sprint(o.fx.Expected["category"]))
		got := o.res.Category
		m.total++
		if m.confusion[want] == nil {
			m.confusion[want] = map[domain.Category]int{}
		}
		m.confusion[want][got]++
		if got == want {
			m.correct++
		} else {
			m.misses = append(m.misses, o)
		}

		lane, _ := enquiry.RouteWith(o.res, threshold)
		wantLane := enquiry.LaneFor(want)
		switch {
		case lane == wantLane:
			m.routedOK++
		case lane == domain.LaneUnsorted:
			m.unsorted++ // safe: the GM sorts it
		default:
			m.misrouted++ // costly: the wrong team owns it
		}
		if want == domain.CategoryComplaint {
			m.complaints++
			if o.res.ComplaintHit {
				m.complaintsCaught++
			}
		} else if o.res.ComplaintHit {
			m.falseAlerts++
		}

		for _, f := range extractedFields {
			exp, ok := o.fx.Expected[f]
			if !ok || exp == nil {
				continue
			}
			m.fieldsTotal[f]++
			if fmt.Sprint(exp) == extractedValue(o.res.Extracted, f) {
				m.fieldsRight[f]++
			}
		}
	}
	return m
}

func extractedValue(e ai.Extracted, field string) string {
	day := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format(time.DateOnly)
	}
	num := func(n *int) string {
		if n == nil {
			return ""
		}
		return fmt.Sprint(*n)
	}
	switch field {
	case "check_in":
		return day(e.CheckIn)
	case "check_out":
		return day(e.CheckOut)
	case "adults":
		return num(e.Adults)
	case "children":
		return num(e.Children)
	case "guest_count":
		return num(e.GuestCount)
	case "event_month":
		if e.EventMonth == nil {
			return ""
		}
		return *e.EventMonth
	}
	return ""
}

func render(runs []run, today time.Time, model string) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("# AI enquiry routing: evaluation\n\n")
	w("Generated by `go run ./cmd/eval` on the 60 labelled enquiries in `apps/api/seed/data/enquiries.json` ")
	w("(reference date %s). Labels were written by hand before the classifier was built; they include the brief's ", today.Format("2 Jan 2006"))
	w("ENQ-4410/4411/4412, Hinglish and Kannada-English messages, a politely disguised complaint, mixed intent, one-word messages and spam.\n\n")
	if model == "" {
		w("> This run had no `GEMINI_API_KEY`, so only the keyword rules were measured. Set the key and re-run to compare.\n\n")
	}
	w("Routing outcomes matter more than raw accuracy. A message routed to the **wrong team** is costly (it sits with people ")
	w("who won't act on it); one sent to **Unsorted** is safe (the GM sorts it in one click). Every complaint must reach the GM.\n\n")

	w("## Summary at the chosen threshold (Unsorted below %.2f)\n\n", ai.UnsortedBelow)
	w("| Classifier | Category accuracy | Routed to the right team | Sent to Unsorted | Routed to the wrong team | Complaints reaching the GM | False complaint alerts |\n")
	w("|---|---|---|---|---|---|---|\n")
	for _, r := range runs {
		m := measure(r, ai.UnsortedBelow)
		w("| %s | %d/%d (%.0f%%) | %d | %d | **%d** | **%d/%d** | %d |\n", r.name, m.correct, m.total, pct(m.correct, m.total),
			m.routedOK, m.unsorted, m.misrouted, m.complaintsCaught, m.complaints, m.falseAlerts)
	}

	w("\n## Choosing the Unsorted threshold\n\n")
	w("Raising the threshold trades wrong-team routings for more work in Unsorted.\n\n")
	w("| Threshold |")
	for _, r := range runs {
		w(" %s: wrong team / Unsorted |", r.name)
	}
	w("\n|---|")
	for range runs {
		w("---|")
	}
	w("\n")
	for _, t := range thresholds {
		w("| %.2f |", t)
		for _, r := range runs {
			m := measure(r, t)
			w(" %d / %d |", m.misrouted, m.unsorted)
		}
		w("\n")
	}

	for _, r := range runs {
		m := measure(r, ai.UnsortedBelow)
		w("\n## %s\n\n### Confusion matrix (rows: label, columns: prediction)\n\n| |", r.name)
		for _, c := range domain.Categories {
			w(" %s |", c)
		}
		w("\n|---|")
		for range domain.Categories {
			w("---|")
		}
		w("\n")
		for _, want := range domain.Categories {
			w("| **%s** |", want)
			for _, got := range domain.Categories {
				w(" %d |", m.confusion[want][got])
			}
			w("\n")
		}
		w("\n### Extraction (where the label has the field)\n\n| Field | Right |\n|---|---|\n")
		for _, f := range extractedFields {
			if m.fieldsTotal[f] > 0 {
				w("| %s | %d/%d |\n", f, m.fieldsRight[f], m.fieldsTotal[f])
			}
		}
		w("\n### Misclassified\n\n")
		if len(m.misses) == 0 {
			w("None.\n")
			continue
		}
		w("| Ref | Label | Predicted | Confidence | Ends up in | Message |\n|---|---|---|---|---|---|\n")
		slices.SortFunc(m.misses, func(a, b outcome) int { return strings.Compare(a.fx.Ref, b.fx.Ref) })
		for _, o := range m.misses {
			lane, _ := enquiry.Route(o.res)
			w("| %s | %v | %s | %.2f | %s | %s |\n", o.fx.Ref, o.fx.Expected["category"], o.res.Category, o.res.Confidence, lane, cell(o.fx.Text))
		}
	}
	w("\n## How to read this\n\n")
	w("- **The keyword score is in-sample and optimistic.** The word lists were written after reading these 60 messages, so they ")
	w("partly memorise this set; on new messages expect noticeably lower accuracy. The model's prompt contains none of these ")
	w("messages, so its score is the fairer estimate. A larger held-out set of real enquiries is the first thing to add in production.\n")
	w("- The keyword rules don't extract dates or months (blank by design); the model does. Staff can correct any extracted field, ")
	w("and the draft's quote is re-priced from the rate table when they do.\n")
	w("- The keyword rules always run, as a cross-check and as the fallback when the model is unavailable, so the first row is the floor the system never drops below.\n")
	w("- A complaint seen by **either** classifier alerts the GM; that is why complaint recall can exceed either classifier's accuracy on its own.\n")
	w("- Prices never come from the model; drafts get them from the rate table, so classification mistakes can't produce a wrong price.\n")
	return b.String()
}

func referenceDay(path string) (time.Time, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	var f struct {
		ReferenceNow time.Time `json:"reference_now"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return time.Time{}, err
	}
	ist := f.ReferenceNow.In(time.FixedZone("IST", 5*3600+1800))
	return time.Date(ist.Year(), ist.Month(), ist.Day(), 0, 0, 0, 0, time.UTC), nil
}

func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 80 {
		s = string(r[:79]) + "…"
	}
	return strings.ReplaceAll(s, "|", "/")
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func envOr(k, v string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return v
}
