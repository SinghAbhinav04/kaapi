package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// google talks to Google's OAuth and Gmail REST endpoints with plain net/http.
type google struct {
	http         *http.Client
	authURL      string
	oauthBase    string
	apiBase      string
	clientID     string
	clientSecret string
	redirectURL  string
}

// statusError is a non-2xx answer; the body is kept short for logs.
type statusError struct {
	Code int
	Body string
}

func (e *statusError) Error() string { return fmt.Sprintf("gmail: HTTP %d: %s", e.Code, e.Body) }

func isStatus(err error, code int) bool {
	var se *statusError
	return errors.As(err, &se) && se.Code == code
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (g *google) consentURL(state string) string {
	q := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {g.redirectURL},
		"response_type": {"code"},
		"scope":         {scopes},
		"access_type":   {"offline"}, // a refresh token, so polling works with nobody signed in
		"prompt":        {"consent"}, // Google only returns a refresh token on consent
		"state":         {state},
	}
	return g.authURL + "?" + q.Encode()
}

func (g *google) exchange(ctx context.Context, code string) (tokenResponse, error) {
	return g.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {g.redirectURL}})
}

func (g *google) refresh(ctx context.Context, refreshToken string) (tokenResponse, error) {
	return g.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
}

func (g *google) token(ctx context.Context, form url.Values) (tokenResponse, error) {
	form.Set("client_id", g.clientID)
	form.Set("client_secret", g.clientSecret)
	var out tokenResponse
	err := g.do(ctx, http.MethodPost, g.oauthBase+"/token", "", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), &out)
	return out, err
}

func (g *google) revoke(ctx context.Context, token string) error {
	return g.do(ctx, http.MethodPost, g.oauthBase+"/revoke", "", "application/x-www-form-urlencoded",
		strings.NewReader(url.Values{"token": {token}}.Encode()), nil)
}

// ---- Gmail

type profile struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    string `json:"historyId"`
}

type historyPage struct {
	History []struct {
		MessagesAdded []struct {
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		} `json:"messagesAdded"`
	} `json:"history"`
	HistoryID     string `json:"historyId"`
	NextPageToken string `json:"nextPageToken"`
}

type messageList struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
	NextPageToken string `json:"nextPageToken"`
}

type part struct {
	MimeType string `json:"mimeType"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		Data string `json:"data"`
	} `json:"body"`
	Parts []part `json:"parts"`
}

type message struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"threadId"`
	LabelIDs []string `json:"labelIds"`
	Payload  part     `json:"payload"`
}

func (g *google) profile(ctx context.Context, access string) (profile, error) {
	var p profile
	return p, g.get(ctx, access, "/gmail/v1/users/me/profile", nil, &p)
}

// history lists messages added to the inbox since a history id. A 404 means the id is too
// old for Google to replay; the caller then falls back to a recent-mail search.
func (g *google) history(ctx context.Context, access, since string) (ids []string, latest string, err error) {
	q := url.Values{"startHistoryId": {since}, "historyTypes": {"messageAdded"}, "labelId": {"INBOX"}}
	latest = since
	for {
		var page historyPage
		if err := g.get(ctx, access, "/gmail/v1/users/me/history", q, &page); err != nil {
			return nil, "", err
		}
		for _, h := range page.History {
			for _, m := range h.MessagesAdded {
				ids = append(ids, m.Message.ID)
			}
		}
		if page.HistoryID != "" {
			latest = page.HistoryID
		}
		if page.NextPageToken == "" {
			return ids, latest, nil
		}
		q.Set("pageToken", page.NextPageToken)
	}
}

// since lists inbox messages that arrived after t (minus our own sent mail), oldest first,
// following every page. Gmail's search takes "after:" as Unix seconds.
func (g *google) since(ctx context.Context, access string, t time.Time) ([]string, error) {
	q := url.Values{"q": {fmt.Sprintf("in:inbox -from:me after:%d", t.Unix())}, "maxResults": {fmt.Sprint(listPageSize)}}
	var ids []string
	for {
		var l messageList
		if err := g.get(ctx, access, "/gmail/v1/users/me/messages", q, &l); err != nil {
			return nil, err
		}
		for _, m := range l.Messages {
			ids = append(ids, m.ID)
		}
		if l.NextPageToken == "" || len(ids) >= maxCatchUp {
			break
		}
		q.Set("pageToken", l.NextPageToken)
	}
	slices.Reverse(ids) // Gmail lists newest first; import in arrival order
	return ids, nil
}

func (g *google) message(ctx context.Context, access, id string) (message, error) {
	var m message
	return m, g.get(ctx, access, "/gmail/v1/users/me/messages/"+url.PathEscape(id), url.Values{"format": {"full"}}, &m)
}

func (g *google) send(ctx context.Context, access, raw, threadID string) (message, error) {
	body := map[string]string{"raw": raw}
	if threadID != "" {
		body["threadId"] = threadID
	}
	b, err := json.Marshal(body)
	if err != nil {
		return message{}, err
	}
	var m message
	return m, g.do(ctx, http.MethodPost, g.apiBase+"/gmail/v1/users/me/messages/send", access, "application/json", bytes.NewReader(b), &m)
}

func (g *google) get(ctx context.Context, access, path string, q url.Values, out any) error {
	u := g.apiBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return g.do(ctx, http.MethodGet, u, access, "", nil, out)
}

func (g *google) do(ctx context.Context, method, u, access, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		return &statusError{Code: res.StatusCode, Body: truncate(string(data), maxErrorRunes)}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
