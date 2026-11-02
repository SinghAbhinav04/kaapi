package ai_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
)

var today = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

func TestRedactRemovesContactDetailsButKeepsBookingFacts(t *testing.T) {
	in := "Hi, I'm Arjun Menon (arjun.menon@example.test, +91 98450 12345). Villa 14-16 Nov for 2 adults, budget 25000."
	out := ai.Redact(in, "Arjun Menon")
	for _, leaked := range []string{"Arjun", "Menon", "arjun.menon", "98450", "12345"} {
		if strings.Contains(out, leaked) {
			t.Errorf("%q leaked: %s", leaked, out)
		}
	}
	for _, kept := range []string{"14-16 Nov", "2 adults", "25000", "[EMAIL]", "[PHONE]", "[NAME]"} {
		if !strings.Contains(out, kept) {
			t.Errorf("%q should survive: %s", kept, out)
		}
	}
}

func TestRedactBareIndianMobile(t *testing.T) {
	if out := ai.Redact("call me on 9845012345 please", ""); strings.Contains(out, "9845012345") {
		t.Fatal(out)
	}
}

func TestKeywordClassifier(t *testing.T) {
	cases := []struct {
		text string
		want domain.Category
	}{
		{"Venue for my sister's wedding in Feb 2027, about 250 guests, 2 nights", domain.CategoryWedding},
		{"Is a villa free 14-16 Nov for 2 adults and a child?", domain.CategoryRoom},
		{"Team offsite for 40 people in January, need a conference hall", domain.CategoryOffsite},
		{"The AC didn't work and the staff were rude. I want a refund.", domain.CategoryComplaint},
		{"Bhai shaadi ke liye venue chahiye, 300 log", domain.CategoryWedding},
		{"We do SEO for hotels, can we talk?", domain.CategoryOther},
		{"hi", domain.CategoryOther},
	}
	for _, c := range cases {
		if got := ai.KeywordClassify(c.text); got.Category != c.want {
			t.Errorf("%q: got %s want %s", c.text, got.Category, c.want)
		}
	}
	if c := ai.KeywordClassify("hi"); c.Confidence >= ai.UnsortedBelow {
		t.Errorf("a one-word message must be unsure, got %.2f", c.Confidence)
	}
	w := ai.KeywordClassify("Venue for my sister's wedding in Feb 2027, about 250 guests, 2 nights")
	if w.Extracted.GuestCount == nil || *w.Extracted.GuestCount != 250 {
		t.Errorf("guest count: %+v", w.Extracted.GuestCount)
	}
}

func TestGuardStripsPricesTheModelWroteItself(t *testing.T) {
	draft := "Hi {{GUEST_NAME}},\n\nThe villa is ₹18,000 a night.\nTotal around Rs. 40000 for two nights.\n{{QUOTE}}\n{{BANK_DETAILS}}\n{{SIGNATURE}}"
	out, reason := ai.Guard(draft)
	for _, bad := range []string{"18,000", "40000", "BANK_DETAILS"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q survived: %s", bad, out)
		}
	}
	if !strings.Contains(out, "{{QUOTE}}") || !strings.Contains(out, "{{SIGNATURE}}") || reason == "" {
		t.Fatalf("out=%q reason=%q", out, reason)
	}
	if _, reason := ai.Guard("Hi {{GUEST_NAME}},\nSee you soon.\n{{SIGNATURE}}"); reason != "" {
		t.Fatalf("clean drafts are left alone: %s", reason)
	}
}

func TestValidateDropsImpossibleExtractions(t *testing.T) {
	past := today.AddDate(0, 0, -3)
	in, out := today.AddDate(0, 0, 5), today.AddDate(0, 0, 4)
	neg, huge := -2, 50_000
	c := ai.Classification{Category: domain.CategoryRoom, Extracted: ai.Extracted{CheckIn: &past, Adults: &neg, GuestCount: &huge}}
	ai.Validate(&c, today)
	if c.Extracted.CheckIn != nil || c.Extracted.Adults != nil || c.Extracted.GuestCount != nil {
		t.Fatalf("%+v", c.Extracted)
	}
	c = ai.Classification{Category: domain.CategoryRoom, Extracted: ai.Extracted{CheckIn: &in, CheckOut: &out}}
	ai.Validate(&c, today)
	if c.Extracted.CheckIn == nil || c.Extracted.CheckOut != nil {
		t.Fatalf("check-out before check-in must be dropped: %+v", c.Extracted)
	}
}

// fakeProvider answers with a fixed classification or error.
type fakeProvider struct {
	c   ai.Classification
	err error
}

func (f fakeProvider) Name() string { return "fake" }
func (f fakeProvider) Classify(context.Context, string, time.Time) (ai.Classification, error) {
	return f.c, f.err
}
func (f fakeProvider) Draft(context.Context, ai.DraftInput) (string, error) { return "", f.err }

func TestClassifierFallsBackWhenTheModelIsDown(t *testing.T) {
	cl := ai.NewClassifier(fakeProvider{err: ai.ErrUnavailable})
	r, err := cl.Classify(context.Background(), "The room was dirty and nobody helped. Refund please.", "", today)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != domain.AIFallback || r.Category != domain.CategoryComplaint || !r.ComplaintHit {
		t.Fatalf("%+v", r)
	}
}

func TestComplaintHitByEitherClassifier(t *testing.T) {
	// The model reads a disguised complaint as a room question; the keyword rules catch it.
	cl := ai.NewClassifier(fakeProvider{c: ai.Classification{Category: domain.CategoryRoom, Confidence: 0.9}})
	r, err := cl.Classify(context.Background(), "Is it normal that we were charged twice for one night? Not happy.", "", today)
	if err != nil {
		t.Fatal(err)
	}
	if !r.ComplaintHit {
		t.Fatalf("a complaint either side sees must alert the GM: %+v", r)
	}
	if r.Confidence >= 0.9 {
		t.Fatalf("disagreement should lower confidence: %.2f", r.Confidence)
	}
}

func TestAgreementRaisesConfidence(t *testing.T) {
	in, out := today.AddDate(0, 0, 12), today.AddDate(0, 0, 14)
	cl := ai.NewClassifier(fakeProvider{c: ai.Classification{Category: domain.CategoryRoom, Confidence: 0.8,
		Extracted: ai.Extracted{CheckIn: &in, CheckOut: &out}}})
	r, _ := cl.Classify(context.Background(), "Is a villa free 14-16 Nov for 2 adults?", "", today)
	if r.Status != domain.AIDone || r.Confidence <= 0.8 {
		t.Fatalf("%+v", r)
	}
}

func TestCanceledContextIsNotAFallback(t *testing.T) {
	cl := ai.NewClassifier(fakeProvider{err: context.Canceled})
	if _, err := cl.Classify(context.Background(), "hello", "", today); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
