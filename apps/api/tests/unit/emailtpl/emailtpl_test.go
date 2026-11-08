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

// The reply from the screenshot of 8 Nov: a lead sentence followed by bullets used to fall
// back to plain paragraphs with "-" dashes. Offers are a bulleted list, prices a price list,
// there's no newsletter headline, and the sender's sign-off closes it.
func TestReplyReadsAsALetterWithListsAndTheSendersSignOff(t *testing.T) {
	body := "Hi Abhinav,\n\n" +
		"For your dates, we have two offers when you book direct on our website:\n" +
		"- Code DIRECT10: 10% off for every room for stays between 1 Nov 2026 and 31 Mar 2027.\n" +
		"- Code STAYLONGER: a fixed amount off the stay for Coffee Villa and Pool Villa, minimum 4 nights.\n\n" +
		"For Mon 9 Nov to Fri 13 Nov (4 nights, 2 adults), totals including GST:\n" +
		"- Plantation Room: ₹31,096 room only, ₹39,412 with breakfast\n" +
		"- Coffee Villa: ₹80,240 room only (2 left)\n\n" +
		"Warm regards,"
	e := emailtpl.Reply("Re: Inquiry about diwali offer for resort", body, emailtpl.SignOff("Abhinav Singh", domain.RoleAdmin), links)

	for _, want := range []string{
		"For your dates, we have two offers", // the lead sentence stays a paragraph
		"&bull;</td>",                        // offers become bullets
		">Plantation Room</p>",               // quote lines become price rows
		">₹31,096 room only, ₹39,412 with breakfast</p>",
		"<strong>Abhinav</strong>", "Kaapi Estate</span>", // the sign-off block
	} {
		if !strings.Contains(e.HTML, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, unwanted := range []string{"- Code DIRECT10", "- Plantation Room", "<h1", "Rohan"} {
		if strings.Contains(e.HTML, unwanted) {
			t.Errorf("should not contain %q", unwanted)
		}
	}
	if !strings.HasSuffix(e.Text, "Warm regards,\nAbhinav\nKaapi Estate") {
		t.Errorf("text part signs after the valediction:\n%s", e.Text)
	}
	writePreview(t, "reply", e.HTML)
}

func TestSignOffNamesTheSenderAndTheirTeam(t *testing.T) {
	cases := map[string]struct {
		name string
		team domain.Role
	}{
		"Rohan\nReservations, Kaapi Estate":               {"Rohan Shetty", domain.RoleReservations},
		"Anitha\nGeneral Manager, Kaapi Estate":           {"Anitha Gowda", domain.RoleGM},
		"Abhinav\nKaapi Estate":                           {"Abhinav Singh", domain.RoleAdmin},
		"Kaapi Estate team\nEvents & Sales, Kaapi Estate": {"", domain.RoleSales},
	}
	for want, c := range cases {
		if got := emailtpl.SignOff(c.name, c.team); got != want {
			t.Errorf("SignOff(%q, %s) = %q, want %q", c.name, c.team, got, want)
		}
	}
}

func TestComplaintAlertLinksToTheInbox(t *testing.T) {
	e := emailtpl.Render(domain.OutboxAlert, "Complaint needs you: ENQ-4471", "A guest complaint arrived.", links)
	if !strings.Contains(e.HTML, `href="https://kaapiestate.test/staff/inbox?open=ENQ-4471"`) || !strings.Contains(e.HTML, "Needs your attention") {
		t.Fatalf("alert has no inbox button:\n%s", e.HTML)
	}
}

func writePreview(t *testing.T, name, html string) {
	if dir := os.Getenv("EMAIL_PREVIEW_DIR"); dir != "" {
		if err := os.WriteFile(dir+"/"+name+".html", []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
