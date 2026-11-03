package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"kaapi/api/internal/platform/httpx"
)

func seenIP(t *testing.T, hops int, remote string, forwarded ...string) string {
	t.Helper()
	var got string
	h := httpx.ClientIP(hops)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
	r := httptest.NewRequest(http.MethodGet, "/v1/rooms", nil)
	r.RemoteAddr = remote
	for _, f := range forwarded {
		r.Header.Add("X-Forwarded-For", f)
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func TestClientIPTrustsOnlyOurProxies(t *testing.T) {
	cases := []struct {
		name      string
		hops      int
		remote    string
		forwarded []string
		want      string
	}{
		{"direct caller", 0, "203.0.113.7:51000", nil, "203.0.113.7"},
		{"direct caller can't claim another address", 0, "203.0.113.7:51000", []string{"1.2.3.4"}, "203.0.113.7"},
		{"behind the web proxy", 1, "127.0.0.1:40000", []string{"203.0.113.7"}, "203.0.113.7"},
		// The client forged the leftmost entry; our proxy appended the real one.
		{"forged entry is ignored", 1, "127.0.0.1:40000", []string{"1.2.3.4, 203.0.113.7"}, "203.0.113.7"},
		{"two proxies", 2, "10.0.0.2:443", []string{"1.2.3.4, 203.0.113.7, 10.0.0.9"}, "203.0.113.7"},
		{"fewer proxies than configured", 2, "203.0.113.7:51000", nil, "203.0.113.7"},
		{"IPv6", 1, "[::1]:40000", []string{"2001:db8::1"}, "2001:db8::1"},
		{"garbage falls back to the connection", 1, "127.0.0.1:40000", []string{"not-an-ip"}, "127.0.0.1"},
	}
	for _, c := range cases {
		if got := seenIP(t, c.hops, c.remote, c.forwarded...); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
