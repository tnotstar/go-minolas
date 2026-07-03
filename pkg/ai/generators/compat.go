package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

const (
	// defaultCompatScheme is the HTTP scheme for compat endpoints. Unlike the
	// local Ollama provider, compat targets are normally served over TLS.
	defaultCompatScheme = "https"

	// defaultCompatPathPrefix is the default path prefix (empty: the caller
	// supplies the full prefix in the URL).
	defaultCompatPathPrefix = ""

	// envCompatAPIKey is the primary env var for the compat provider.
	envCompatAPIKey = "COMPAT_API_KEY"
)

// CompatOpener implements the Opener interface for any OpenAI Chat
// Completions-format endpoint whose host differs from api.openai.com (e.g.
// OpenRouter, vLLM, LM Studio, Together, Groq, DeepSeek). It supports the
// "compat" URL scheme.
//
// URL format: compat://[userinfo@]{host}[:port]/{path_prefix...}/{model}
//
// Both the host and the model (the last path segment) are REQUIRED; opening a
// compat URL without either returns an error. The API key is resolved from the
// COMPAT_API_KEY environment variable, falling back to OPENAI_API_KEY.
//
// Examples:
//   - compat://localhost:1234/v1/custom-model
//   - compat://api.openrouter.ai/api/v1/openai/gpt-4o
//   - compat://user:pass@my-vllm:8000/v1/llama3
type CompatOpener struct{}

// Id returns the unique identifier for the compat opener.
func (o *CompatOpener) Id() string { return "compat" }

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "compat" scheme.
func (o *CompatOpener) CanOpen(u *url.URL) bool { return u.Scheme == "compat" }

// Open creates a compat generator client using the provided URL. Unlike the
// openai opener, both the host and the model must be present in the URL.
func (o *CompatOpener) Open(_ context.Context, u *url.URL) (Generator, error) {
	if u == nil {
		return nil, errors.New("generators: URL cannot be nil for CompatOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("generators: scheme %q not supported by CompatOpener (expected compat)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("generators: compat URL must include a host (e.g. compat://localhost:1234/model)")
	}

	pathPrefix, model := splitLastSegment(u.EscapedPath())
	if model == "" {
		return nil, errors.New("generators: compat URL must include a model as the last path segment (e.g. compat://host/prefix/model)")
	}

	basePath := ""
	if pathPrefix != "" {
		basePath = "/" + pathPrefix
	}
	base := &url.URL{
		Scheme: defaultCompatScheme,
		Host:   u.Host,
		Path:   basePath,
	}
	if u.User != nil {
		base.User = u.User
	}

	return &CompatGenerator{
		httpClient: &http.Client{},
		apiKey:     lookupCompatAPIKey(),
		model:      model,
		baseURL:    base.String(),
	}, nil
}

func init() {
	RegisterOpener(&CompatOpener{})
}

// lookupCompatAPIKey resolves the API key: COMPAT_API_KEY takes precedence,
// falling back to OPENAI_API_KEY so consumers can reuse an existing key.
func lookupCompatAPIKey() string {
	if key := os.Getenv(envCompatAPIKey); key != "" {
		return key
	}
	return os.Getenv(envOpenAIAPIKey)
}

// --- CompatGenerator ---

// CompatGenerator implements the Generator interface for any OpenAI Chat
// Completions-format endpoint, using the shared chat-completion wire-format
// helpers.
type CompatGenerator struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
}

// Generate produces a text completion for the given prompt via the compat
// endpoint's /v1/chat/completions.
func (g *CompatGenerator) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	cfg := newConfig(opts)
	cfg.Model = g.resolveModel(cfg)

	reqBody := buildChatCompletionRequest(cfg, prompt)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: compat marshal request: %w", err)
	}

	endpoint := g.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: compat create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: compat request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("generators: compat read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generators: compat API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parseChatCompletionResponse(respBody, cfg.Model)
}

// Stream produces a streaming text completion via the compat endpoint's
// /v1/chat/completions with SSE.
func (g *CompatGenerator) Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error) {
	cfg := newConfig(opts)
	cfg.Model = g.resolveModel(cfg)

	reqBody := buildChatCompletionRequest(cfg, prompt)
	reqBody.Stream = true
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: compat marshal request: %w", err)
	}

	endpoint := g.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: compat create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: compat stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("generators: compat API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		consumeChatCompletionSSE(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// Close releases the resources held by the compat generator.
func (g *CompatGenerator) Close() error { return nil }

// resolveModel returns the model from the config if set, otherwise the default.
func (g *CompatGenerator) resolveModel(cfg *Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return g.model
}
