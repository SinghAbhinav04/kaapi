package api

import (
	"net/http"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/platform/httpx"
)

func userDTO(u auth.User) UserDTO {
	return UserDTO{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, Permissions: auth.PermissionsOf(u.Role)}
}

// signedIn writes the user, and either sets httpOnly cookies (web) or returns the tokens
// in the body (native app, which keeps them in secure storage).
func (a *API) signedIn(w http.ResponseWriter, r *http.Request, u auth.User, t auth.Tokens) {
	out := AuthResponse{User: userDTO(u)}
	if isNative(r) {
		out.AccessToken, out.AccessExpiresAt, out.RefreshToken = &t.Access, &t.AccessExpiresAt, &t.Refresh
	} else {
		a.setAuthCookies(w, t)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) staffLogin(w http.ResponseWriter, r *http.Request) error {
	var body StaffLoginRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	u, t, err := a.auth.StaffLogin(r.Context(), body.Email, body.Password, clientInfo(r))
	if err != nil {
		return err
	}
	a.signedIn(w, r, u, t)
	return nil
}

func (a *API) otpRequest(w http.ResponseWriter, r *http.Request) error {
	var body OTPRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	code, err := a.auth.RequestOTP(r.Context(), body.Email)
	if err != nil {
		return err
	}
	out := OTPRequestResponse{Sent: true}
	if a.cfg.DemoMode {
		out.DemoCode = &code
	}
	httpx.WriteJSON(w, http.StatusAccepted, out)
	return nil
}

func (a *API) otpVerify(w http.ResponseWriter, r *http.Request) error {
	var body OTPVerifyRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	u, t, err := a.auth.VerifyOTP(r.Context(), body.Email, body.Code, clientInfo(r))
	if err != nil {
		return err
	}
	a.signedIn(w, r, u, t)
	return nil
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) error {
	token := ""
	if isNative(r) {
		var body RefreshRequest
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return err
		}
		token = body.RefreshToken
	} else if c, err := r.Cookie(a.refreshCookie()); err == nil {
		token = c.Value
	}
	u, t, err := a.auth.Refresh(r.Context(), token, clientInfo(r))
	if err != nil {
		a.clearAuthCookies(w)
		return err
	}
	a.signedIn(w, r, u, t)
	return nil
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) error {
	if p := principal(r.Context()); p.IsAuthenticated() {
		if err := a.auth.Logout(r.Context(), p.SessionID); err != nil {
			return err
		}
	}
	a.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *API) me(w http.ResponseWriter, r *http.Request) error {
	u, err := a.auth.Me(r.Context(), principal(r.Context()).UserID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]UserDTO{"user": userDTO(u)})
	return nil
}

func (a *API) listOutbox(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	limit := p.Int("limit", outbox.DefaultLimit, 1, outbox.MaxLimit)
	if err := p.Err(); err != nil {
		return err
	}
	msgs, err := a.outbox.List(r.Context(), limit)
	if err != nil {
		return err
	}
	out := OutboxResponse{Messages: make([]OutboxMessageDTO, len(msgs))}
	for i, m := range msgs {
		out.Messages[i] = outboxDTO(m)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}
