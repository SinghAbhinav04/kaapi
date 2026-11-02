package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Groq calls one model on Groq's OpenAI-compatible API. It's the second line after the
// Gemini models: a different company's infrastructure, so one outage can't take both.
type Groq struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

const groqBaseURL = "https://api.groq.com/openai"

func NewGroq(apiKey, model, baseURL string) *Groq {
	if baseURL == "" {
		baseURL = groqBaseURL
	}
	return &Groq{apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: providerTimeout}}
}

func (g *Groq) Name() string { return "groq:" + g.model }

func (g *Groq) Classify(ctx context.Context, message string, today time.Time) (Classification, error) {
	raw, err := g.chat(ctx, classifyPrompt(today)+jsonShape, message, 0, true)
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

func (g *Groq) Draft(ctx context.Context, in DraftInput) (string, error) {
	return g.chat(ctx, draftSystem, draftUserMessage(in), draftTemperature, false)
}

func (g *Groq) chat(ctx context.Context, system, user string, temperature float64, jsonMode bool) (string, error) {
	req := map[string]any{
		"model":       g.model,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	// Reasoning models think before answering; keep it short, the task is simple.
	switch {
	case strings.HasPrefix(g.model, "openai/gpt-oss"):
		req["reasoning_effort"] = "low"
	case strings.HasPrefix(g.model, "qwen/qwen3"):
		req["reasoning_effort"] = "none"
	}
	if jsonMode {
		req["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	res, raw, err := send(g.http, httpReq)
	if err != nil {
		return "", &ProviderError{Model: g.Name(), Kind: FailUnavailable, Err: err}
	}
	if res.StatusCode != http.StatusOK {
		retry, _ := strconv.Atoi(res.Header.Get("Retry-After"))
		return "", &ProviderError{Model: g.Name(), Kind: failKindForStatus(res.StatusCode), RetryAfter: time.Duration(retry) * time.Second,
			Err: fmt.Errorf("HTTP %d: %.*s", res.StatusCode, errorSnippetBytes, raw)}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", &ProviderError{Model: g.Name(), Kind: FailBadOutput, Err: err}
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", &ProviderError{Model: g.Name(), Kind: FailBadOutput, Err: errors.New("empty answer")}
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
