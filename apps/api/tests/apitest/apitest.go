// Package apitest runs the real router against a seeded database and checks every
// response against contract/openapi.yaml, so the contract can't drift from the code.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/api"
	"kaapi/api/internal/platform/config"
	"kaapi/api/internal/platform/httpx"
	"kaapi/api/tests/testdb"
)

// Today is the date the seed data was written for.
var Today = func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }

// TestConfig is a complete development config with throwaway secrets.
func TestConfig() config.Config {
	return config.Config{
		AppOrigin:      "http://localhost:3000",
		JWESecret:      []byte("test-only-jwe-secret-0123456789abcdef"),
		PaymentSecret:  []byte("test-only-payment-secret-0123456789ab"),
		Environment:    config.EnvDevelopment,
		RequestTimeout: 15 * time.Second,
	}
}

type Server struct {
	t      *testing.T
	Ctx    context.Context
	Pool   *pgxpool.Pool
	URL    string
	client *http.Client
	router routers.Router
}

// New seeds the database and starts the API.
func New(t *testing.T) *Server {
	t.Helper()
	ctx, pool := testdb.Seeded(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(api.New(TestConfig(), log, pool, Today).Router())
	t.Cleanup(srv.Close)
	return &Server{t: t, Ctx: ctx, Pool: pool, URL: srv.URL, client: srv.Client(), router: loadContract(t)}
}

func loadContract(t *testing.T) routers.Router {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "contract", "openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("contract is not valid OpenAPI: %v", err)
	}
	doc.Servers = nil // match paths on any host
	r, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Response is a finished request.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into v, failing the test if it can't.
func (r Response) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

// Error decodes an error envelope.
func (r Response) Error(t *testing.T) httpx.ErrorPayload {
	t.Helper()
	var b httpx.ErrorBody
	r.JSON(t, &b)
	return b.Error
}

// Do sends a request and validates the response against the contract. Routes that aren't
// in the contract (deliberate 404/405 probes) are only checked for the error envelope.
func (s *Server) Do(method, path string, body any, headers map[string]string) Response {
	s.t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			s.t.Fatal(err)
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(s.Ctx, method, s.URL+path, buf)
	if err != nil {
		s.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := s.client.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	out := Response{Status: res.StatusCode, Header: res.Header, Body: raw}
	if res.Header.Get(httpx.HeaderRequestID) == "" {
		s.t.Errorf("%s %s: no request id header", method, path)
	}
	s.validate(req, out)
	return out
}

func (s *Server) validate(req *http.Request, res Response) {
	s.t.Helper()
	route, params, err := s.router.FindRoute(req)
	if err != nil {
		if res.Status < 400 {
			s.t.Errorf("%s %s answered %d but is not in the contract", req.Method, req.URL.Path, res.Status)
		}
		return
	}
	in := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{IncludeResponseStatus: true}}
	err = openapi3filter.ValidateResponse(s.Ctx, &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in, Status: res.Status, Header: res.Header, Body: io.NopCloser(bytes.NewReader(res.Body)),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	})
	if err != nil {
		s.t.Errorf("%s %s → %d breaks the contract: %v\nbody: %s", req.Method, req.URL.Path, res.Status, err, res.Body)
	}
}
