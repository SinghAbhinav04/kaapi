package emailtpl_test

import (
	"os"
	"strings"
	"testing"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/emailtpl"
)

var links = emailtpl.Links{Site: "https://kaapiestate.test", Directions: "https://maps.example.test/estate"}

func TestCodeGetsItsOwnBox(t *testing.T) {
	e := emailtpl.Render(domain.OutboxOTP, "Your Kaapi Estate sign-in code",
		"Your sign-in code is 482913. It expires in 10 minutes. If you didn't ask for it, you can ignore this email.", links)
	if !strings.Contains(e.HTML, ">482913<") || strings.Contains(e.HTML, "is 482913") || !strings.Contains(e.HTML, "It expires in 10 minutes.") {
		t.Fatalf("code not boxed:\n%s", e.HTML)
	}
	writePreview(t, "otp", e.HTML)
}

func TestBookingGetsBadgeButtonAndList(t *testing.T) {
	body := "Thank you, Arjun. Your booking KS-7Q3M9X is confirmed.\n\nCoffee Villa, Breakfast included\nSat 14 Nov to Mon 16 Nov 2026 (2 nights), 1 room(s)\n\n" +
		"- Coffee Villa: ₹44,368 room only, ₹47,200 with breakfast\n- Pool Villa: ₹60,888 room only"
	e := emailtpl.Render(domain.OutboxBookingConfirmation, "Booking confirmed: KS-7Q3M9X", body, links)
	for _, want := range []string{">KS-7Q3M9X<", `href="https://kaapiestate.test/booking/KS-7Q3M9X"`, "View your booking", ">Pool Villa<"} {
		if !strings.Contains(e.HTML, want) {
			t.Errorf("missing %q", want)
		}
	}
	writePreview(t, "booking", e.HTML)
}

// A guest's own words are text, never markup: a reply quoting them can't inject HTML.
func TestTextIsEscaped(t *testing.T) {
	e := emailtpl.Render(domain.OutboxEnquiryReply, `Re: <b>hi</b>`, `Hi <script>alert(1)</script> see https://x.test/?a=1&b=2`, links)
	if strings.Contains(e.HTML, "<script>") || strings.Contains(e.HTML, "<b>hi</b>") {
		t.Fatal("unescaped markup in the email")
	}
	if !strings.Contains(e.HTML, `href="https://x.test/?a=1&amp;b=2"`) {
		t.Fatal("bare link not linked")
	}
	if e.Text == "" || e.HTML == "" {
		t.Fatal("both parts must be present")
	}
}

func writePreview(t *testing.T, name, html string) {
	if dir := os.Getenv("EMAIL_PREVIEW_DIR"); dir != "" {
		if err := os.WriteFile(dir+"/"+name+".html", []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
