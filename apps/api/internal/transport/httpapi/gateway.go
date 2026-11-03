package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

// Cookie names. In production the __Host- / __Secure- prefixes make the browser refuse
// them over plain HTTP or from another (sub)domain; local development uses plain names.
const (
	cookieAccessSecure  = "__Host-kaapi_at"
	cookieRefreshSecure = "__Secure-kaapi_rt"
	cookieAccessDev     = "kaapi_at"
	cookieRefreshDev    = "kaapi_rt"

	headerAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
	headerClient        = "X-Client"
	clientNative        = "native"
	headerRetryAfter    = "Retry-After"
)

type principalKey struct{}
type tokenRejectedKey struct{}

func (a *API) accessCookie() string {
	if a.cfg.CookieSecure {
		return cookieAccessSecure
	}
	return cookieAccessDev
}

// refreshCookiePath limits the refresh token to the auth routes, as the browser addresses
// them (through the web app's proxy): with the bare /v1/auth the browser never sends it back.
func (a *API) refreshCookiePath() string { return a.cfg.BrowserAPIPrefix + "/v1/auth" }

func (a *API) refreshCookie() string {
	if a.cfg.CookieSecure {
		return cookieRefreshSecure
	}
	return cookieRefreshDev
}

// The gateway, in order (router.go): CORS for the request's group → authenticate →
// rateLimit → checkOrigin. Routes add only their permission (requirePermission).

// authenticate reads a token from the Bearer header (native app) or the access cookie (web),
// decrypts it, checks its session is still live (not logged out, rotated or revoked, and the
// user still active) and puts the Principal on the context. Requests without a valid token
// continue as anonymous; routes that need more say so with requirePermission.
func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if h := r.Header.Get(headerAuthorization); strings.HasPrefix(h, bearerPrefix) {
			token = strings.TrimPrefix(h, bearerPrefix)
		} else if c, err := r.Cookie(a.accessCookie()); err == nil {
			token = c.Value
		}
		ctx := r.Context()
		if token != "" {
			claims, err := a.auth.Sealer().Open(token)
			live := false
			if err == nil {
				if live, err = a.auth.SessionLive(ctx, claims.SessionID); err != nil {
					a.log.ErrorContext(ctx, "session check failed", "err", err) // fail closed: treated as signed out
				}
			}
			if live {
				ctx = context.WithValue(ctx, principalKey{}, auth.Principal{UserID: claims.UserID, Role: claims.Role, SessionID: claims.SessionID})
			} else {
				ctx = context.WithValue(ctx, tokenRejectedKey{}, true)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// principal returns the caller; the zero Principal means anonymous.
func principal(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey{}).(auth.Principal)
	return p
}

// requireAuth rejects anonymous callers. A caller whose token was present but expired gets
// a distinct message, which tells the client to refresh rather than show a sign-in page.
func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !principal(r.Context()).IsAuthenticated() {
			err := auth.ErrUnauthenticated
			if rejected, _ := r.Context().Value(tokenRejectedKey{}).(bool); rejected {
				err = auth.ErrTokenInvalid
			}
			httpx.WriteError(a.log, w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// can reports whether the caller holds a permission, as the database grants it now (a
// short cache in package access). Anonymous callers hold none.
func (a *API) can(r *http.Request, p auth.Permission) bool {
	who := principal(r.Context())
	if !who.IsAuthenticated() {
		return false
	}
	ok, err := a.access.Can(r.Context(), who.UserID, p)
	if err != nil {
		a.log.ErrorContext(r.Context(), "permission lookup failed", "err", err)
	}
	return ok
}

// requirePermission lets a route through only for people whose role grants the permission.
func (a *API) requirePermission(p auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !a.can(r, p) {
				httpx.WriteError(a.log, w, r, auth.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// rateLimit counts the request against its group's rule (policies.go): by account for signed-in
// callers where the group says so, otherwise by IP (already the trusted client address, see
// httpx.ClientIP). Over the limit: 429 RATE_LIMITED with Retry-After. A counter outage lets
// requests through rather than locking everyone out.
func (a *API) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g := groupFor(r)
		if !a.cfg.RateLimits || g == nil || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		subject := a.limits.Subject("ip", r.RemoteAddr)
		if who := principal(r.Context()); g.key == byUserElseIP && who.IsAuthenticated() {
			subject = a.limits.Subject("user", who.UserID.String())
		}
		d, err := a.limits.Allow(r.Context(), g.rule, subject)
		if err != nil {
			a.log.ErrorContext(r.Context(), "rate limit check failed", "group", g.name, "err", err)
		}
		if !d.Allowed {
			secs := int(math.Ceil(d.RetryAfter.Seconds()))
			w.Header().Set(headerRetryAfter, strconv.Itoa(secs))
			httpx.WriteError(a.log, w, r, apperr.New(apperr.CodeRateLimited,
				fmt.Sprintf("Too many requests. Please try again in %s.", waitText(secs))).With(apperr.Details{"retry_after_seconds": secs}))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func waitText(secs int) string {
	if secs < 60 {
		return strconv.Itoa(secs) + " seconds"
	}
	return strconv.Itoa((secs+59)/60) + " minutes"
}

// checkOrigin blocks cross-site writes that ride on our cookies (CSRF). Browsers always
// send Origin on POST/PUT/PATCH/DELETE; non-browser clients use Bearer tokens instead.
func (a *API) checkOrigin(next http.Handler) http.Handler {
	allowed := append([]string{a.cfg.AppOrigin}, a.cfg.NativeAppOrigins...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if unsafe && a.hasAuthCookie(r) {
			if origin := r.Header.Get("Origin"); origin != "" && !slices.Contains(allowed, origin) {
				httpx.WriteError(a.log, w, r, auth.ErrBadOrigin)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) hasAuthCookie(r *http.Request) bool {
	_, e1 := r.Cookie(a.accessCookie())
	_, e2 := r.Cookie(a.refreshCookie())
	return e1 == nil || e2 == nil
}

func (a *API) setAuthCookies(w http.ResponseWriter, t auth.Tokens) {
	// Max-Age is relative, so a skewed server or client clock can't expire a cookie early.
	http.SetCookie(w, &http.Cookie{Name: a.accessCookie(), Value: t.Access, Path: "/", MaxAge: int(auth.AccessTTL.Seconds()),
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: a.refreshCookie(), Value: t.Refresh, Path: a.refreshCookiePath(), MaxAge: int(auth.RefreshTTL.Seconds()),
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteStrictMode})
}

func (a *API) clearAuthCookies(w http.ResponseWriter) {
	past := time.Unix(0, 0)
	http.SetCookie(w, &http.Cookie{Name: a.accessCookie(), Value: "", Path: "/", Expires: past, MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: a.refreshCookie(), Value: "", Path: a.refreshCookiePath(), Expires: past, MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteStrictMode})
}

// isNative reports whether the caller asked for tokens in the body (the Capacitor app).
func isNative(r *http.Request) bool { return r.Header.Get(headerClient) == clientNative }

func clientInfo(r *http.Request) auth.ClientInfo {
	return auth.ClientInfo{UserAgent: r.UserAgent(), IP: r.RemoteAddr}
}
