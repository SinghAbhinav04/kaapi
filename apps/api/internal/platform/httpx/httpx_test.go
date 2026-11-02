package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/platform/apperr"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func serve(h http.Handler, req *http.Request) (*httptest.ResponseRecorder, ErrorBody) {
	rec := httptest.NewRecorder()
	RequestID(Recover(quiet)(h)).ServeHTTP(rec, req)
	var body ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestErrorsBecomeTheEnvelope(t *testing.T) {
	h := Handle(quiet, func(w http.ResponseWriter, r *http.Request) error {
		return apperr.New(apperr.CodeSoldOut, "No rooms left.").With(apperr.Details{"nights": []string{"2026-12-30"}})
	})
	rec, body := serve(h, httptest.NewRequest(http.MethodPost, "/holds", nil))
	if rec.Code != http.StatusConflict || body.Error.Code != apperr.CodeSoldOut || body.Error.Details["nights"] == nil {
		t.Fatalf("%d %+v", rec.Code, body)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != rec.Header().Get(HeaderRequestID) {
		t.Fatal("the request ID must be in the body and the header")
	}
}

func TestUnknownErrorIsAGeneric500(t *testing.T) {
	h := Handle(quiet, func(w http.ResponseWriter, r *http.Request) error { return errors.New("dial tcp 10.0.0.5: refused") })
	rec, body := serve(h, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if body.Error.Code != apperr.CodeInternal {
		t.Fatal(body.Error.Code)
	}
}

func TestPanicIsRecovered(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("nil map") })
	rec, body := serve(h, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError || body.Error.Code != apperr.CodeInternal || body.Error.RequestID == "" {
		t.Fatalf("%d %+v", rec.Code, body)
	}
}

func TestRequestIDIsReusedWhenSane(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "edge-abc123")
	rec, _ := serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), req)
	if rec.Header().Get(HeaderRequestID) != "edge-abc123" {
		t.Fatal("incoming request ID should be kept")
	}
	bad := httptest.NewRequest(http.MethodGet, "/", nil)
	bad.Header.Set(HeaderRequestID, "evil\r\nSet-Cookie: x")
	rec, _ = serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), bad)
	if strings.ContainsAny(rec.Header().Get(HeaderRequestID), "\r\n ") {
		t.Fatal("unsafe request ID was echoed")
	}
}

func TestDecodeJSON(t *testing.T) {
	type in struct {
		Rooms int `json:"rooms"`
	}
	cases := []struct {
		body string
		code apperr.Code
	}{
		{``, apperr.CodeBadRequest},
		{`{"rooms":`, apperr.CodeBadRequest},
		{`{"rooms":"two"}`, apperr.CodeValidation},
		{`{"rooms":2,"admin":true}`, apperr.CodeValidation},
		{`{"rooms":2}{"rooms":3}`, apperr.CodeBadRequest},
		{`{"rooms":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, apperr.CodePayloadTooLarge},
	}
	for _, tc := range cases {
		var v in
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		err := DecodeJSON(httptest.NewRecorder(), req, &v)
		if got := apperr.From(err); got == nil || got.Code != tc.code {
			t.Errorf("%.40q: got %v, want %s", tc.body, err, tc.code)
		}
	}
	var ok in
	if err := DecodeJSON(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"rooms":2}`)), &ok); err != nil || ok.Rooms != 2 {
		t.Fatal(err)
	}
}
