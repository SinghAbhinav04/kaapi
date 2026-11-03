package httpx

import (
	"net"
	"net/http"
	"strings"
)

const headerForwardedFor = "X-Forwarded-For"

// ClientIP sets r.RemoteAddr to the caller's address, trusting only the proxies we run.
//
// Each proxy appends the address it received the request from to X-Forwarded-For, so with
// `hops` proxies in front of the API, the caller is the hops-th address from the right of
// [X-Forwarded-For…, RemoteAddr]. Anything further left was written by the client and can
// be forged, which is why chi's RealIP (leftmost entry) isn't used: a bot could rotate fake
// addresses and walk past every per-IP limit.
//
// hops: 0 when callers reach the API directly, 1 behind the web app's /api proxy, 2 behind
// Vercel plus Cloud Run's front end.
func ClientIP(hops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = clientIP(r, hops)
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request, hops int) string {
	chain := []string{}
	for _, h := range r.Header.Values(headerForwardedFor) {
		for _, part := range strings.Split(h, ",") {
			if ip := strings.TrimSpace(part); ip != "" {
				chain = append(chain, ip)
			}
		}
	}
	chain = append(chain, hostOnly(r.RemoteAddr))
	i := len(chain) - 1 - hops
	if i < 0 {
		i = 0 // fewer proxies than configured: the leftmost is still the best we have
	}
	if ip := net.ParseIP(hostOnly(chain[i])); ip != nil {
		return ip.String()
	}
	return hostOnly(r.RemoteAddr)
}

func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(addr, "[]")
}
