package outbox

import "strings"

// Reserved reports whether an address is on a domain reserved for documentation and testing
// (RFC 2606 and 6761): no mail sent there can ever arrive.
func Reserved(addr string) bool {
	at := strings.LastIndexByte(addr, '@')
	if at < 0 {
		return true
	}
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(addr[at+1:])), ".")
	for _, tld := range []string{".test", ".example", ".invalid", ".localhost"} {
		if strings.HasSuffix(domain, tld) {
			return true
		}
	}
	switch domain {
	case "example.com", "example.net", "example.org":
		return true
	}
	return strings.HasSuffix(domain, ".example.com") || strings.HasSuffix(domain, ".example.net") || strings.HasSuffix(domain, ".example.org")
}
