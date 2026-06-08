package generators

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	// defaultGeminiScheme is the HTTP scheme for the Gemini API.
	defaultGeminiScheme = "https"

	// defaultGeminiHost is the default host for the Gemini REST API.
	defaultGeminiHost = "generativelanguage.googleapis.com"

	// defaultGeminiPathPrefix is the default path prefix for the Gemini REST API.
	defaultGeminiPathPrefix = "v1beta/models"

	// defaultGeminiModel is the default model used when none is specified in the URL.
	defaultGeminiModel = "gemini-2.0-flash"

	// envGeminiAPIKey is the primary environment variable for the Gemini API key.
	envGeminiAPIKey = "GEMINI_API_KEY"

	// envGoogleAPIKey is the fallback environment variable for the Google API key.
	envGoogleAPIKey = "GOOGLE_API_KEY"
)

// GeminiOpener implements the Opener interface for the Google Gemini API.
// It supports the "gemini" URL scheme and uses the Gemini REST API directly.
//
// URL format: gemini://[userinfo@]{host}[:port]/{path_prefix...}/{model}
//
// The scheme identifies the Gemini provider. The netloc and path prefix (minus
// the last segment) form the base URL. The last path segment is the model name.
// API keys are read exclusively from environment variables.
//
// Examples:
//   - gemini://                                                            (defaults)
//   - gemini:///gemini-2.0-pro                                             (default host+prefix, specific model)
//   - gemini://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash (fully specified)
type GeminiOpener struct{}

// Id returns the unique identifier for the Gemini opener.
func (o *GeminiOpener) Id() string {
	return "gemini"
}

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "gemini" scheme.
func (o *GeminiOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "gemini"
}

// Open creates a Gemini generator client using the provided URL.
// API key is read from the GEMINI_API_KEY or GOOGLE_API_KEY environment variable.
func (o *GeminiOpener) Open(_ context.Context, u *url.URL) (Generator, error) {
	if u == nil {
		return nil, errors.New("generators: URL cannot be nil for GeminiOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("generators: scheme %q not supported by GeminiOpener (expected gemini)", u.Scheme)
	}

	cfg := parseProviderURL(u, defaultGeminiScheme, defaultGeminiHost, defaultGeminiPathPrefix, defaultGeminiModel)

	return &GeminiGenerator{
		httpClient: &http.Client{},
		apiKey:     lookupGeminiAPIKey(),
		model:      cfg.model,
		baseURL:    cfg.baseURL,
	}, nil
}

func init() {
	RegisterOpener(&GeminiOpener{})
}

// lookupGeminiAPIKey reads the API key from environment variables.
// Priority: GEMINI_API_KEY > GOOGLE_API_KEY.
func lookupGeminiAPIKey() string {
	if key := os.Getenv(envGeminiAPIKey); key != "" {
		return key
	}
	return os.Getenv(envGoogleAPIKey)
}

// --- Internal JSON types for the Gemini REST API ---

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature     *float32 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            *float32 `json:"topP,omitempty"`
	TopK            *float32 `json:"topK,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// --- GeminiGenerator ---

// GeminiGenerator implements the Generator interface for Google Gemini
// using the REST API directly via net/http.
type GeminiGenerator struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
}

// Generate produces a text completion for the given prompt using the Gemini REST API.
func (g *GeminiGenerator) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	cfg := newConfig(opts)
	reqBody := g.buildRequestBody(cfg, prompt)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: gemini marshal request: %w", err)
	}

	model := g.resolveModel(cfg)
	endpoint := fmt.Sprintf("%s/%s:generateContent", g.baseURL, model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: gemini create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("generators: gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var gemResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gemResp); err != nil {
		return nil, fmt.Errorf("generators: gemini decode response: %w", err)
	}

	return g.parseResponse(&gemResp, model), nil
}

// Stream produces a streaming text completion for the given prompt using the Gemini REST API.
// Returns a read-only channel that yields response chunks as they arrive via SSE.
func (g *GeminiGenerator) Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error) {
	cfg := newConfig(opts)
	reqBody := g.buildRequestBody(cfg, prompt)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: gemini marshal request: %w", err)
	}

	model := g.resolveModel(cfg)
	endpoint := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse", g.baseURL, model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: gemini create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: gemini stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("generators: gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)
		defer resp.Body.Close()
		g.consumeSSE(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// Close releases the resources held by the Gemini generator.
func (g *GeminiGenerator) Close() error {
	return nil
}

// resolveModel returns the model from the config if set, otherwise the default.
func (g *GeminiGenerator) resolveModel(cfg *Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return g.model
}

// buildRequestBody converts a Config and prompt into a Gemini API request.
func (g *GeminiGenerator) buildRequestBody(cfg *Config, prompt string) geminiRequest {
	req := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	genCfg := &geminiGenConfig{}
	hasConfig := false

	if cfg.Temperature != 0 {
		genCfg.Temperature = ptrFloat32(cfg.Temperature)
		hasConfig = true
	}
	if cfg.MaxOutputTokens != 0 {
		genCfg.MaxOutputTokens = cfg.MaxOutputTokens
		hasConfig = true
	}
	if cfg.TopP != 0 {
		genCfg.TopP = ptrFloat32(cfg.TopP)
		hasConfig = true
	}
	if cfg.TopK != 0 {
		genCfg.TopK = ptrFloat32(cfg.TopK)
		hasConfig = true
	}
	if len(cfg.StopSequences) > 0 {
		genCfg.StopSequences = cfg.StopSequences
		hasConfig = true
	}
	if hasConfig {
		req.GenerationConfig = genCfg
	}

	if cfg.SystemInstruction != "" {
		req.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: cfg.SystemInstruction}},
		}
	}

	return req
}

// parseResponse converts a geminiResponse into a generators.Response.
func (g *GeminiGenerator) parseResponse(resp *geminiResponse, model string) *Response {
	out := &Response{
		Model: model,
	}

	if resp == nil {
		return out
	}

	if len(resp.Candidates) > 0 {
		c := resp.Candidates[0]
		var texts []string
		for _, p := range c.Content.Parts {
			if p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		out.Text = strings.Join(texts, "")
		out.FinishReason = c.FinishReason
	}

	if resp.UsageMetadata != nil {
		out.Usage = Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return out
}

// consumeSSE reads Server-Sent Events from the response body and sends
// parsed chunks on the channel.
func (g *GeminiGenerator) consumeSSE(ctx context.Context, body io.Reader, ch chan<- StreamChunk) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			ch <- StreamChunk{Error: ctx.Err()}
			return
		}

		line := scanner.Text()

		// SSE lines start with "data: " prefix.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		var gemResp geminiResponse
		if err := json.Unmarshal([]byte(data), &gemResp); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("generators: gemini SSE unmarshal: %w", err)}
			return
		}

		if len(gemResp.Candidates) > 0 {
			for _, p := range gemResp.Candidates[0].Content.Parts {
				if p.Text != "" {
					ch <- StreamChunk{Text: p.Text}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("generators: gemini SSE read: %w", err)}
	}
}

// ptrFloat32 returns a pointer to the given float32 value.
func ptrFloat32(v float32) *float32 {
	return &v
}
