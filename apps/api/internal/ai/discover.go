package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Options configures the model chain. Empty model lists mean "ask the provider which
// models this key can use".
type Options struct {
	GeminiKey     string
	GeminiBaseURL string
	GeminiModels  []string
	GeminiRPM     int

	GroqKey     string
	GroqBaseURL string
	GroqModels  []string
	GroqRPM     int
}

// Fallback lists, used if listing models fails at start-up.
var (
	defaultGeminiModels = []string{"gemini-2.5-flash", "gemini-2.5-flash-lite"}
	defaultGroqModels   = []string{"openai/gpt-oss-120b", "openai/gpt-oss-20b"}
)

var (
	// Text models only, and no "-latest" aliases (they share quota with a named model).
	geminiFastRe = regexp.MustCompile(`^gemini-(\d+(?:\.\d+)?)-flash(?:-lite)?(?:-preview)?$`)
	geminiProRe  = regexp.MustCompile(`^gemini-(\d+(?:\.\d+)?)-pro(?:-preview)?$`)
	groqChatRe   = regexp.MustCompile(`^(openai/gpt-oss-\d+b|qwen/qwen3[\w.-]*|llama-3\.3-70b-versatile|meta-llama/llama-4-[\w-]+|moonshotai/kimi-k2[\w-]*)$`)
	groqSkipRe   = regexp.MustCompile(`guard|safeguard|whisper|orpheus|tts`)
)

// Build assembles the chain: fast Gemini models, then Gemini Pro, then Groq. It returns
// nil when no key is configured, and the problems it worked around (for the log).
func Build(ctx context.Context, o Options) (*Chain, []string) {
	var warnings []string
	var fast, pro, groq []Provider
	if o.GeminiKey != "" {
		names := o.GeminiModels
		if len(names) == 0 {
			listed, err := listGemini(ctx, o.GeminiKey, o.GeminiBaseURL)
			if err != nil || len(listed) == 0 {
				warnings = append(warnings, fmt.Sprintf("listing Gemini models failed (%v); using %v", err, defaultGeminiModels))
				listed = defaultGeminiModels
			}
			names = listed
		}
		for _, n := range names {
			p := NewGemini(o.GeminiKey, n, o.GeminiBaseURL)
			if geminiProRe.MatchString(n) {
				pro = append(pro, p)
			} else {
				fast = append(fast, p)
			}
		}
	}
	if o.GroqKey != "" {
		names := o.GroqModels
		if len(names) == 0 {
			listed, err := listGroq(ctx, o.GroqKey, o.GroqBaseURL)
			if err != nil || len(listed) == 0 {
				warnings = append(warnings, fmt.Sprintf("listing Groq models failed (%v); using %v", err, defaultGroqModels))
				listed = defaultGroqModels
			}
			names = listed
		}
		for _, n := range names {
			groq = append(groq, NewGroq(o.GroqKey, n, o.GroqBaseURL))
		}
	}
	c := NewChain(
		Tier{Name: "gemini", Providers: fast, RPM: o.GeminiRPM},
		Tier{Name: "gemini-pro", Providers: pro, RPM: o.GeminiRPM},
		Tier{Name: "groq", Providers: groq, RPM: o.GroqRPM},
	)
	if c.Empty() {
		return nil, warnings
	}
	return c, warnings
}

// listGemini returns the text models this key can call, newest first.
func listGemini(ctx context.Context, key, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = geminiBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1beta/models?pageSize=1000", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(geminiKeyHeader, key)
	var out struct {
		Models []struct {
			Name    string   `json:"name"`
			Methods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := getJSON(req, &out); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range out.Models {
		n := strings.TrimPrefix(m.Name, "models/")
		if slices.Contains(m.Methods, "generateContent") && (geminiFastRe.MatchString(n) || geminiProRe.MatchString(n)) {
			names = append(names, n)
		}
	}
	slices.SortStableFunc(names, func(a, b string) int { return compareVersions(geminiVersion(b), geminiVersion(a)) })
	return names, nil
}

// listGroq returns the chat models this key can call.
func listGroq(ctx context.Context, key, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = groqBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	var out struct {
		Data []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"data"`
	}
	if err := getJSON(req, &out); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range out.Data {
		if m.Active && groqChatRe.MatchString(m.ID) && !groqSkipRe.MatchString(m.ID) {
			names = append(names, m.ID)
		}
	}
	slices.Sort(names)
	return names, nil
}

func getJSON(req *http.Request, into any) error {
	res, raw, err := send(&http.Client{Timeout: providerTimeout}, req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %.*s", res.StatusCode, errorSnippetBytes, raw)
	}
	return json.Unmarshal(raw, into)
}

func geminiVersion(name string) string {
	if m := geminiFastRe.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	if m := geminiProRe.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return "0"
}

// compareVersions compares dotted numbers: "3.10" > "3.9".
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(as), len(bs)); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}
