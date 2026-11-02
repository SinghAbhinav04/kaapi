package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Gemini calls one Gemini model through the REST API: one dependency fewer than the SDK,
// and the request is plain to read. The key travels in a header, never in the URL, so it
// can't end up in access logs. The Chain spreads work across several of these.
type Gemini struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

const (
	geminiBaseURL     = "https://generativelanguage.googleapis.com"
	geminiKeyHeader   = "x-goog-api-key"
	geminiRetryInfo   = "type.googleapis.com/google.rpc.RetryInfo"
	providerTimeout   = 25 * time.Second
	maxResponseBytes  = 1 << 20
	draftTemperature  = 0.4
	errorSnippetBytes = 200
)

func NewGemini(apiKey, model, baseURL string) *Gemini {
	if baseURL == "" {
		baseURL = geminiBaseURL
	}
	return &Gemini{apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: providerTimeout}}
}

func (g *Gemini) Name() string { return "gemini:" + g.model }

func (g *Gemini) Classify(ctx context.Context, message string, today time.Time) (Classification, error) {
	raw, err := g.generate(ctx, classifyPrompt(today), message, map[string]any{
		"temperature": 0, "responseMimeType": "application/json", "responseSchema": classifySchema(),
	})
	if err != nil {
		return Classification{}, err
	}
	c, err := decodeClassification(raw)
	if err != nil {
		return Classification{}, &ProviderError{Model: g.Name(), Kind: FailBadOutput, Err: err}
	}
	c.AnsweredBy = g.Name()
	return c, nil
}

func (g *Gemini) Draft(ctx context.Context, in DraftInput) (string, error) {
	return g.generate(ctx, draftSystem, draftUserMessage(in), map[string]any{"temperature": draftTemperature})
}

func (g *Gemini) generate(ctx context.Context, system, user string, config map[string]any) (string, error) {
	body, err := json.Marshal(map[string]any{
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": system}}},
		"contents":          []map[string]any{{"role": "user", "parts": []map[string]string{{"text": user}}}},
		"generationConfig":  config,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, g.model), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(geminiKeyHeader, g.apiKey)
	res, raw, err := send(g.http, req)
	if err != nil {
		return "", &ProviderError{Model: g.Name(), Kind: FailUnavailable, Err: err}
	}
	if res.StatusCode != http.StatusOK {
		return "", &ProviderError{Model: g.Name(), Kind: failKindForStatus(res.StatusCode), RetryAfter: geminiRetryAfter(raw),
			Err: fmt.Errorf("HTTP %d: %.*s", res.StatusCode, errorSnippetBytes, raw)}
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", &ProviderError{Model: g.Name(), Kind: FailBadOutput, Err: err}
	}
	if len(out.Candidates) == 0 {
		return "", &ProviderError{Model: g.Name(), Kind: FailBadOutput, Err: errors.New("no answer (blocked or empty)")}
	}
	var text strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		if !p.Thought {
			text.WriteString(p.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", &ProviderError{Model: g.Name(), Kind: FailBadOutput, Err: errors.New("empty answer")}
	}
	return strings.TrimSpace(text.String()), nil
}

// geminiRetryAfter reads the RetryInfo Google attaches to a 429 ("retryDelay": "37s").
func geminiRetryAfter(raw []byte) time.Duration {
	var body struct {
		Error struct {
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return 0
	}
	for _, d := range body.Error.Details {
		if d.Type == geminiRetryInfo {
			if v, err := time.ParseDuration(d.RetryDelay); err == nil {
				return v
			}
		}
	}
	return 0
}

// send does the request and reads a bounded body.
func send(c *http.Client, req *http.Request) (*http.Response, []byte, error) {
	res, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	return res, raw, err
}
