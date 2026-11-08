// Package emailtpl turns an email's plain text into the resort's branded HTML. Every email
// the system sends (codes, welcome, confirmations, reminders, refunds, staff replies) goes
// out as multipart/alternative: this HTML and the original text, so clients that can't show
// HTML still read it. Messages are stored as text; the layout is applied when sending, so
// one change here restyles everything.
//
// The HTML is email-safe: tables and inline styles only (Gmail strips <style>), web-safe
// fonts, a 560 px column, and every piece of text escaped by html/template.
package emailtpl

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"

	"kaapi/api/internal/domain"
)

// Links are the site addresses buttons point at.
type Links struct {
	Site       string // e.g. https://kaapiestate.in
	Directions string // a maps link to the estate
}

// Email is one rendered message.
type Email struct {
	Subject string
	Text    string
	HTML    string
}

// teamName closes every sign-off.
const teamName = "Kaapi Estate"

// SignOff is how a person signs what they send: first name, then their team. Administrators
// (and anyone without a team) sign for the estate.
func SignOff(name string, team domain.Role) string {
	first := teamName + " team"
	if f := strings.Fields(name); len(f) > 0 {
		first = f[0]
	}
	if title := team.Title(); title != "" {
		return first + "\n" + title + ", " + teamName
	}
	return first + "\n" + teamName
}

var (
	codeRe   = regexp.MustCompile(`\b(\d{6})\b`)
	refRe    = regexp.MustCompile(`\bKS-[2-9A-HJ-NP-Z]{6}\b`)
	enqRe    = regexp.MustCompile(`\bENQ-\d+\b`)
	urlRe    = regexp.MustCompile(`https?://[^\s<>"]+`)
	bulletRe = regexp.MustCompile(`^[-•*]\s+(.+)$`)
	// A price line from the rate table: "Plantation Room: ₹31,096 room only, …".
	priceLine = regexp.MustCompile(`^(.+?): (.*₹.*)$`)
)

// block is one piece of the body, in order.
type block struct {
	Kind  string // "p", "code", "prices", "bullets", "button"
	Lines []string
	Items []item
	Code  string
	Href  string
	Label string
}

type item struct{ Name, Detail string }

type page struct {
	Subject   string
	Eyebrow   string
	Badge     string
	Letter    bool     // a person's message: no headline, closed by their sign-off
	SignOff   []string // the first line is the name
	Blocks    []block
	Site      string
	Preheader string
}

// Reply builds a message a person wrote and signs: a reply to an enquiry or a staff email.
// It reads as a letter (no headline, which made a reply look like a newsletter) and the
// sender's sign-off closes it, in both parts.
func Reply(subject, body, signOff string, links Links) Email {
	p := page{Subject: subject, Site: strings.TrimRight(links.Site, "/"), Letter: true, Blocks: paragraphs(strings.TrimSpace(body))}
	if signOff = strings.TrimSpace(signOff); signOff != "" {
		p.SignOff = strings.Split(signOff, "\n")
	}
	return execute(p, subject, Signed(body, signOff))
}

// Signed is the plain text of a signed message: the body, then the sign-off.
func Signed(body, signOff string) string {
	text, signOff := strings.TrimSpace(body), strings.TrimSpace(signOff)
	switch {
	case signOff == "":
		return text
	case strings.HasSuffix(text, ","): // "Warm regards," reads straight into the name
		return text + "\n" + signOff
	default:
		return text + "\n\n" + signOff
	}
}

// Render builds the HTML for a message of the given kind.
func Render(kind domain.OutboxKind, subject, text string, links Links) Email {
	if kind == domain.OutboxEnquiryReply || kind == domain.OutboxStaffMessage {
		return Reply(subject, text, "", links) // already signed: stored copies include the sign-off
	}
	p := page{Subject: subject, Site: strings.TrimRight(links.Site, "/"), Eyebrow: eyebrow(kind)}
	ref := refRe.FindString(subject + " " + text)

	switch kind {
	case domain.OutboxOTP, domain.OutboxStaffCode:
		// The code gets its own box; the sentence it came from stays as text.
		if c := codeRe.FindStringSubmatch(text); c != nil {
			p.Blocks = append(p.Blocks, block{Kind: "code", Code: c[1]})
			text = dropSentenceWith(text, c[1]) // the box says it; the rest (expiry, safety) stays
		}
	case domain.OutboxBookingConfirmation, domain.OutboxCheckInReminder, domain.OutboxCheckOutReminder, domain.OutboxRefundNotice:
		p.Badge = ref
	}
	p.Blocks = append(p.Blocks, paragraphs(text)...)

	switch {
	case ref != "" && p.Site != "" && kind != domain.OutboxRefundNotice:
		p.Blocks = append(p.Blocks, block{Kind: "button", Href: p.Site + "/booking/" + ref, Label: "View your booking"})
	case kind == domain.OutboxWelcome && p.Site != "":
		p.Blocks = append(p.Blocks, block{Kind: "button", Href: p.Site + "/#stays", Label: "See the rooms"})
	case kind == domain.OutboxAlert && p.Site != "":
		if enq := enqRe.FindString(subject); enq != "" {
			p.Blocks = append(p.Blocks, block{Kind: "button", Href: p.Site + "/staff/inbox?open=" + enq, Label: "Open it in the inbox"})
		}
	}
	if kind == domain.OutboxCheckInReminder && links.Directions != "" {
		p.Blocks = append(p.Blocks, block{Kind: "button", Href: links.Directions, Label: "Directions to the estate"})
	}
	return execute(p, subject, text)
}

func execute(p page, subject, text string) Email {
	p.Preheader = preheader(p.Blocks)
	var b bytes.Buffer
	if err := layout.Execute(&b, p); err != nil {
		// The template is fixed and the data escaped; fall back to text rather than fail a send.
		return Email{Subject: subject, Text: text}
	}
	return Email{Subject: subject, Text: text, HTML: b.String()}
}

// paragraphs splits text on blank lines, and each paragraph into runs of plain lines and
// bullet lines ("- ", "• " or "* "), so "Totals including GST:" followed by bullets is a lead
// sentence and a list. A run where every bullet carries a rupee price (a quote from the rate
// table) becomes a price list; any other run a bulleted list.
func paragraphs(text string) []block {
	var out []block
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		var lines, bullets []string
		flush := func() {
			if len(lines) > 0 {
				out = append(out, block{Kind: "p", Lines: lines})
			}
			if len(bullets) > 0 {
				out = append(out, list(bullets))
			}
			lines, bullets = nil, nil
		}
		for _, l := range strings.Split(para, "\n") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if m := bulletRe.FindStringSubmatch(l); m != nil {
				if len(lines) > 0 {
					flush()
				}
				bullets = append(bullets, m[1])
				continue
			}
			if len(bullets) > 0 {
				flush()
			}
			lines = append(lines, l)
		}
		flush()
	}
	return out
}

func list(bullets []string) block {
	prices := block{Kind: "prices"}
	for _, b := range bullets {
		m := priceLine.FindStringSubmatch(b)
		if m == nil {
			plain := block{Kind: "bullets"}
			for _, b := range bullets {
				plain.Items = append(plain.Items, item{Detail: b})
			}
			return plain
		}
		prices.Items = append(prices.Items, item{Name: m[1], Detail: m[2]})
	}
	return prices
}

// dropSentenceWith removes the sentence containing s ("Your code is 123456. It expires…").
func dropSentenceWith(text, s string) string {
	i := strings.Index(text, s)
	start := strings.LastIndex(text[:i], ". ") + 1
	end := strings.Index(text[i:], ". ")
	if end < 0 {
		return strings.TrimSpace(text[:start])
	}
	return strings.TrimSpace(text[:start] + text[i+end+1:])
}

func preheader(blocks []block) string {
	for _, b := range blocks {
		if b.Kind == "p" {
			s := strings.Join(b.Lines, " ")
			if r := []rune(s); len(r) > 110 {
				s = string(r[:110]) + "…"
			}
			return s
		}
	}
	return ""
}

func eyebrow(kind domain.OutboxKind) string {
	switch kind {
	case domain.OutboxOTP:
		return "Your sign-in code"
	case domain.OutboxStaffCode:
		return "Staff sign-in"
	case domain.OutboxWelcome:
		return "Welcome"
	case domain.OutboxBookingConfirmation:
		return "Booking confirmed"
	case domain.OutboxCheckInReminder:
		return "See you soon"
	case domain.OutboxCheckOutReminder:
		return "Check-out today"
	case domain.OutboxRefundNotice:
		return "About your payment"
	case domain.OutboxPromotion:
		return "Offers for you"
	case domain.OutboxAlert:
		return "Needs your attention"
	default:
		return "Kaapi Estate"
	}
}

// linkify escapes a line and turns bare URLs into links.
func linkify(s string) template.HTML {
	var b strings.Builder
	last := 0
	for _, m := range urlRe.FindAllStringIndex(s, -1) {
		b.WriteString(template.HTMLEscapeString(s[last:m[0]]))
		u := template.HTMLEscapeString(s[m[0]:m[1]])
		b.WriteString(`<a href="` + u + `" style="color:#3f6b4a;text-decoration:underline;">` + u + `</a>`)
		last = m[1]
	}
	b.WriteString(template.HTMLEscapeString(s[last:]))
	return template.HTML(b.String()) //nolint:gosec // every piece above is escaped
}

var layout = template.Must(template.New("email").Funcs(template.FuncMap{"linkify": linkify}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light only"><title>{{.Subject}}</title></head>
<body style="margin:0;padding:0;background:#f5f1ea;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;">{{.Preheader}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f5f1ea;">
<tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;">
<tr><td style="padding:0 4px 20px 4px;font-family:Georgia,'Times New Roman',serif;font-size:22px;letter-spacing:2px;color:#141210;font-weight:bold;">KAAPI ESTATE</td></tr>
<tr><td style="background:#fffdf9;border:1px solid #e3dccf;border-radius:16px;padding:32px;">
{{if not .Letter}}<p style="margin:0 0 6px 0;font-family:'Courier New',monospace;font-size:12px;letter-spacing:1.5px;text-transform:uppercase;color:#3f6b4a;">{{.Eyebrow}}</p>
<h1 style="margin:0 0 20px 0;font-family:Georgia,'Times New Roman',serif;font-size:26px;line-height:1.25;font-weight:normal;color:#141210;">{{.Subject}}</h1>{{end}}
{{if .Badge}}<p style="margin:0 0 20px 0;"><span style="display:inline-block;padding:6px 12px;border-radius:999px;background:#dfe8de;font-family:'Courier New',monospace;font-size:14px;letter-spacing:1px;color:#2c4a35;">{{.Badge}}</span></p>{{end}}
{{range .Blocks}}
{{if eq .Kind "code"}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:4px 0 22px 0;"><tr><td style="background:#141210;border-radius:12px;padding:18px 28px;font-family:'Courier New',monospace;font-size:34px;letter-spacing:10px;color:#f5f1ea;font-weight:bold;">{{.Code}}</td></tr></table>{{end}}
{{if eq .Kind "p"}}<p style="margin:0 0 16px 0;font-family:Helvetica,Arial,sans-serif;font-size:15px;line-height:1.6;color:#3a3631;">{{range $i, $l := .Lines}}{{if $i}}<br>{{end}}{{linkify $l}}{{end}}</p>{{end}}
{{if eq .Kind "prices"}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 20px 0;border-top:1px solid #ece6db;">{{range .Items}}<tr><td style="padding:10px 0;border-bottom:1px solid #ece6db;"><p style="margin:0;font-family:Helvetica,Arial,sans-serif;font-size:14px;line-height:1.4;color:#141210;font-weight:bold;">{{.Name}}</p><p style="margin:2px 0 0 0;font-family:'Courier New',monospace;font-size:13px;line-height:1.5;color:#3a3631;">{{.Detail}}</p></td></tr>{{end}}</table>{{end}}
{{if eq .Kind "bullets"}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 16px 0;">{{range .Items}}<tr><td valign="top" width="18" style="padding:0 0 8px 2px;font-family:Helvetica,Arial,sans-serif;font-size:15px;line-height:1.6;color:#3f6b4a;">&bull;</td><td style="padding:0 0 8px 0;font-family:Helvetica,Arial,sans-serif;font-size:15px;line-height:1.6;color:#3a3631;">{{linkify .Detail}}</td></tr>{{end}}</table>{{end}}
{{if eq .Kind "button"}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:8px 0 8px 0;"><tr><td style="border-radius:999px;background:#2f7a4b;"><a href="{{.Href}}" style="display:inline-block;padding:12px 22px;font-family:Helvetica,Arial,sans-serif;font-size:14px;font-weight:bold;color:#ffffff;text-decoration:none;">{{.Label}}</a></td></tr></table>{{end}}
{{end}}
{{with .SignOff}}<p style="margin:24px 0 0 0;padding-top:16px;border-top:1px solid #ece6db;font-family:Helvetica,Arial,sans-serif;font-size:15px;line-height:1.5;color:#141210;">{{range $i, $l := .}}{{if $i}}<br><span style="font-size:13px;color:#6b655e;">{{$l}}</span>{{else}}<strong>{{$l}}</strong>{{end}}{{end}}</p>{{end}}
</td></tr>
<tr><td style="padding:20px 4px 0 4px;font-family:Helvetica,Arial,sans-serif;font-size:12px;line-height:1.6;color:#6b655e;">
Kaapi Estate, Baba Budan Hills, Chikkamagaluru, Karnataka.<br>
Check-in from 2 pm, check-out by 11 am. We never ask for passport details by email.{{if .Site}}<br>
<a href="{{.Site}}" style="color:#6b655e;">{{.Site}}</a> &middot; <a href="{{.Site}}/privacy" style="color:#6b655e;">Privacy</a>{{end}}
</td></tr>
</table></td></tr></table></body></html>`))
