package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"kaapi/api/internal/gmail"
	"kaapi/api/internal/outbox"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

// Where the browser lands after Google's consent screen.
const gmailReturnPath = "/staff/inbox"

// gmailStatus: GET /v1/staff/integrations/gmail
func (a *API) gmailStatus(w http.ResponseWriter, r *http.Request) error {
	st, err := a.gmail.Status(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, GmailStatusResponse{Gmail: gmailStatusDTO(st)})
	return nil
}

// gmailConnect: POST /v1/staff/integrations/gmail/connect → the Google consent URL to open.
func (a *API) gmailConnect(w http.ResponseWriter, r *http.Request) error {
	u, err := a.gmail.ConsentURL(principal(r.Context()).UserID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, GmailConnectResponse{AuthURL: u})
	return nil
}

// gmailCallback: GET /v1/integrations/gmail/callback — Google sends the browser here. It's a
// page visit, not an API call, so it always answers with a redirect back to the inbox.
func (a *API) gmailCallback(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	back := url.Values{}
	switch {
	case q.Get("error") != "":
		back.Set("gmail", "cancelled")
	default:
		email, err := a.gmail.Callback(r.Context(), q.Get("code"), q.Get("state"))
		if err != nil {
			a.log.Warn("gmail connect failed", "err", err)
			back.Set("gmail", "error")
			back.Set("reason", apperr.From(err).Message)
		} else {
			back.Set("gmail", "connected")
			back.Set("account", email)
			go func() { // pick up anything that arrived during the round trip
				ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Minute)
				defer cancel()
				_, _ = a.gmail.Poll(ctx)
			}()
		}
	}
	http.Redirect(w, r, a.cfg.AppOrigin+gmailReturnPath+"?"+back.Encode(), http.StatusFound)
	return nil
}

// gmailDisconnect: DELETE /v1/staff/integrations/gmail
func (a *API) gmailDisconnect(w http.ResponseWriter, r *http.Request) error {
	if err := a.gmail.Disconnect(r.Context()); err != nil {
		return err
	}
	return a.gmailStatus(w, r)
}

// gmailSync: POST /v1/staff/integrations/gmail/sync → poll now instead of waiting for the tick.
func (a *API) gmailSync(w http.ResponseWriter, r *http.Request) error {
	n, err := a.gmail.Poll(r.Context())
	if err != nil {
		return err
	}
	st, err := a.gmail.Status(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, GmailSyncResponse{Imported: n, Gmail: gmailStatusDTO(st)})
	return nil
}

// composeMessage: POST /v1/staff/messages — a new email written by staff.
func (a *API) composeMessage(w http.ResponseWriter, r *http.Request) error {
	var body ComposeRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	m, err := a.outbox.Send(r.Context(), principal(r.Context()).UserID, outbox.Compose{To: body.To, Subject: body.Subject, Body: body.Body})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, OutboxMessageResponse{Message: outboxDTO(m)})
	return nil
}

func gmailStatusDTO(s gmail.Status) GmailStatusDTO {
	return GmailStatusDTO{Configured: s.Configured, Connected: s.Connected, AccountEmail: nilIfBlank(s.AccountEmail),
		ConnectedAt: s.ConnectedAt, LastSyncAt: s.LastSyncAt, LastError: s.LastError}
}

func outboxDTO(m outbox.Message) OutboxMessageDTO {
	return OutboxMessageDTO{ID: m.ID, Channel: m.Channel, Kind: m.Kind, Status: m.Status, Recipient: m.Recipient,
		Subject: m.Subject, Body: m.Body, CreatedAt: m.CreatedAt}
}

func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
