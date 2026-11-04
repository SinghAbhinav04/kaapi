package gmail

import (
	"bytes"
	"encoding/base64"
	"html"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"

	"kaapi/api/internal/domain"
)

var (
	wordDecoder = new(mime.WordDecoder)
	tagRe       = regexp.MustCompile(`(?s)<(script|style)[^>]*>.*?</(script|style)>|<[^>]+>`)
	blankRunsRe = regexp.MustCompile(`\n{3,}`)
	// "On Tue, 3 Nov 2026 at 10:02, Asha <asha@example.com> wrote:" starts the quoted history.
	quoteIntroRe = regexp.MustCompile(`(?m)^On .{6,200} wrote:\s*$`)
)

// parse turns a Gmail message into an inbound enquiry: sender, subject, and the new text
// only (quoted history removed, HTML reduced to text).
func parse(m message) Inbound {
	in := Inbound{MessageID: m.ID, ThreadID: m.ThreadID}
	for _, h := range m.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			if a, err := mail.ParseAddress(h.Value); err == nil {
				in.FromName, in.FromEmail = a.Name, strings.ToLower(a.Address)
			}
		case "subject":
			if s, err := wordDecoder.DecodeHeader(h.Value); err == nil {
				in.Subject = s
			} else {
				in.Subject = h.Value
			}
		case "message-id":
			in.RFCMessageID = strings.TrimSpace(h.Value)
		}
	}
	text := findBody(m.Payload, "text/plain")
	if text == "" {
		text = htmlToText(findBody(m.Payload, "text/html"))
	}
	in.Text = truncate(stripQuoted(text), maxBodyRunes)
	return in
}

func findBody(p part, mimeType string) string {
	if strings.HasPrefix(p.MimeType, mimeType) && p.Body.Data != "" {
		return decodeData(p.Body.Data)
	}
	for _, c := range p.Parts {
		if s := findBody(c, mimeType); s != "" {
			return s
		}
	}
	return ""
}

// decodeData reads Gmail's base64url body, with or without padding.
func decodeData(s string) string {
	if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return string(b)
	}
	return ""
}

func htmlToText(s string) string {
	s = strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n\n", "</div>", "\n").Replace(s)
	return html.UnescapeString(tagRe.ReplaceAllString(s, ""))
}

// stripQuoted keeps what the guest wrote now, not the thread below it.
func stripQuoted(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if loc := quoteIntroRe.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), ">") {
			keep = append(keep, strings.TrimRight(line, " \t"))
		}
	}
	return strings.TrimSpace(blankRunsRe.ReplaceAllString(strings.Join(keep, "\n"), "\n\n"))
}

// build writes an RFC 5322 message as Gmail's send API wants it: base64url of the raw bytes.
func build(from string, m domain.Mail) (string, error) {
	var b bytes.Buffer
	header := func(k, v string) { b.WriteString(k + ": " + clean(v) + "\r\n") }
	header("From", from)
	header("To", m.To)
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	if m.InReplyTo != "" {
		header("In-Reply-To", m.InReplyTo)
		header("References", m.InReplyTo)
	}
	header("MIME-Version", "1.0")
	if m.HTML == "" {
		header("Content-Type", `text/plain; charset="UTF-8"`)
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := writeQP(&b, strings.ReplaceAll(m.Body, "\n", "\r\n")); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b.Bytes()), nil
	}
	// Text and HTML of the same message; clients show the last part they can render.
	const boundary = "kaapi-alt-6f1d2c"
	header("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", strings.ReplaceAll(m.Body, "\n", "\r\n")}, {"text/html", m.HTML}} {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString(`Content-Type: ` + part.kind + `; charset="UTF-8"` + "\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQP(&b, part.body); err != nil {
			return "", err
		}
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return base64.RawURLEncoding.EncodeToString(b.Bytes()), nil
}

func writeQP(b *bytes.Buffer, s string) error {
	qp := quotedprintable.NewWriter(b)
	if _, err := qp.Write([]byte(s)); err != nil {
		return err
	}
	return qp.Close()
}

// clean stops header injection: a value can never start a new header line.
func clean(v string) string { return strings.NewReplacer("\r", " ", "\n", " ").Replace(v) }
