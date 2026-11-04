package httpapi

import (
	"net/http"
	"strings"
	"time"

	"kaapi/api/internal/platform/ratelimit"
)

// The gateway's policies, one row per API group: who may call it cross-origin, and how
// often. Routes don't repeat any of this; the gateway picks the first group that matches.
// The numbers are assumptions sized for one resort's traffic, to revisit with real logs.

// corsScope says which origins a group answers cross-origin.
type corsScope int

const (
	corsNone   corsScope = iota // server-to-server or top-level navigation: no CORS at all
	corsApp                     // the web app's origin only
	corsClient                  // the web app and the native shells
)

// rateKey says whose requests a rule counts together.
type rateKey int

const (
	byIP         rateKey = iota // anonymous callers can only be told apart by address
	byUserElseIP                // signed-in callers by account, so a shared office IP isn't one bucket
)

type group struct {
	name  string
	match func(method, path string) bool
	cors  corsScope
	rule  ratelimit.Rule
	key   rateKey
}

func post(path string) func(string, string) bool {
	return func(m, p string) bool { return m == http.MethodPost && p == path }
}

func under(prefix string) func(string, string) bool {
	return func(_, p string) bool { return strings.HasPrefix(p, prefix) }
}

// Shared rules count in the database (every API instance together): sign-in attempts and
// anything that locks inventory or creates records. The rest count per process.
var groups = []group{
	{name: "payment-webhook", match: post("/v1/payments/webhook"), cors: corsNone, key: byIP,
		rule: ratelimit.Rule{Name: "webhook", Limit: 600, Window: time.Minute}},
	{name: "gmail-return", match: func(m, p string) bool { return m == http.MethodGet && p == "/v1/integrations/gmail/callback" },
		cors: corsNone, key: byIP, rule: ratelimit.Rule{Name: "oauth-return", Limit: 30, Window: 10 * time.Minute}},

	{name: "staff-login", match: post("/v1/auth/staff/login"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "staff-login", Limit: 10, Window: 15 * time.Minute, Shared: true}},
	{name: "staff-verify", match: post("/v1/auth/staff/verify"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "staff-verify", Limit: 20, Window: 10 * time.Minute, Shared: true}},
	{name: "otp-request", match: post("/v1/auth/otp/request"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "otp-request", Limit: 10, Window: 10 * time.Minute, Shared: true}},
	{name: "otp-verify", match: post("/v1/auth/otp/verify"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "otp-verify", Limit: 20, Window: 10 * time.Minute, Shared: true}},
	{name: "refresh", match: post("/v1/auth/refresh"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "refresh", Limit: 60, Window: time.Minute}},

	// A hold locks a room for ten minutes, so a bot holding everything is the attack to stop.
	{name: "holds", match: post("/v1/holds"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "holds", Limit: 20, Window: 10 * time.Minute, Shared: true}},
	{name: "bookings", match: post("/v1/bookings"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "bookings", Limit: 20, Window: 10 * time.Minute, Shared: true}},
	{name: "enquiries", match: post("/v1/enquiries"), cors: corsClient, key: byIP,
		rule: ratelimit.Rule{Name: "enquiries", Limit: 20, Window: time.Hour, Shared: true}},
	{name: "payment-simulate", match: func(m, p string) bool {
		return m == http.MethodPost && strings.HasPrefix(p, "/v1/payments/") && strings.HasSuffix(p, "/simulate")
	}, cors: corsClient, key: byIP, rule: ratelimit.Rule{Name: "payment-simulate", Limit: 30, Window: 10 * time.Minute}},

	{name: "staff", match: under("/v1/staff/"), cors: corsApp, key: byUserElseIP,
		rule: ratelimit.Rule{Name: "staff", Limit: 600, Window: time.Minute}},

	// Everything else: the public catalogue, holds and bookings reads, the guest's own pages.
	{name: "public", match: under("/v1/"), cors: corsClient, key: byUserElseIP,
		rule: ratelimit.Rule{Name: "public", Limit: 300, Window: time.Minute}},
}

func groupFor(r *http.Request) *group {
	method := r.Method
	// A preflight asks about the method that will follow; judge it as that request.
	if asked := r.Header.Get("Access-Control-Request-Method"); method == http.MethodOptions && asked != "" {
		method = asked
	}
	for i := range groups {
		if groups[i].match(method, r.URL.Path) {
			return &groups[i]
		}
	}
	return nil
}

// originsFor is the CORS policy: the origins a request's group answers.
func (a *API) originsFor(r *http.Request) []string {
	g := groupFor(r)
	if g == nil {
		return nil
	}
	switch g.cors {
	case corsApp:
		return []string{a.cfg.AppOrigin}
	case corsClient:
		return append([]string{a.cfg.AppOrigin}, a.cfg.NativeAppOrigins...)
	default:
		return nil
	}
}
