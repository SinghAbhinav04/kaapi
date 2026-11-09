package domain

import "strings"

// Mail is a message to send from the resort's connected mailbox. InReplyTo (the guest's
// Message-ID) and ThreadID make it a reply in the guest's existing conversation.
type Mail struct {
	To        string
	Subject   string
	Body      string
	HTML      string // optional: sent with Body as multipart/alternative
	InReplyTo string
	ThreadID  string
}

// SentMail is what the mail provider returned for a delivered message.
type SentMail struct {
	MessageID string
	ThreadID  string
}

// ReservedAddress reports whether an address is on a domain reserved for documentation and testing
// (RFC 2606 and 6761): no mail sent there can ever arrive, so every send path records it as
// simulated instead (the seed uses them; bounces would land back in the inbox).
func ReservedAddress(addr string) bool {
	at := strings.LastIndexByte(addr, '@')
	if at < 0 {
		return true
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(addr[at+1:])), ".")
	for _, tld := range []string{".test", ".example", ".invalid", ".localhost"} {
		if strings.HasSuffix(host, tld) {
			return true
		}
	}
	switch host {
	case "example.com", "example.net", "example.org":
		return true
	}
	return strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".example.net") || strings.HasSuffix(host, ".example.org")
}
