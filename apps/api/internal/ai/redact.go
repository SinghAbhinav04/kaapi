// Package ai reads enquiries: it removes personal data, classifies the message with an
// LLM and with keyword rules, checks and blends the two, and writes reply drafts whose
// prices are filled in by the server, never by the model.
package ai

import (
	"regexp"
	"strings"
)

// Tokens that replace personal data before any text leaves our servers (DPDP: send only
// what the task needs). The model sees "[PHONE]", never the number.
const (
	tokenEmail = "[EMAIL]"
	tokenPhone = "[PHONE]"
	tokenName  = "[NAME]"
)

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// Indian mobiles (+91, 0, or bare 10 digits starting 6-9) and other international numbers.
	phoneRe = regexp.MustCompile(`(?:\+?\d{1,3}[\s\-]?)?(?:\(?\d{2,5}\)?[\s\-]?)?\d{3,5}[\s\-]?\d{4,5}`)
)

// Redact removes emails, phone numbers and the sender's name (each word of it, 3+ letters).
func Redact(text, senderName string) string {
	out := emailRe.ReplaceAllString(text, tokenEmail)
	out = phoneRe.ReplaceAllStringFunc(out, func(m string) string {
		digits := 0
		for _, r := range m {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits >= 10 { // shorter runs are dates, counts or prices, which the classifier needs
			return tokenPhone
		}
		return m
	})
	for _, part := range strings.Fields(senderName) {
		if len([]rune(part)) < 3 {
			continue
		}
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(part) + `\b`)
		out = re.ReplaceAllString(out, tokenName)
	}
	return out
}
