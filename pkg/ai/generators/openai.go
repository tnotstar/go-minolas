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
	// defaultOpenAIScheme is the HTTP scheme for the OpenAI API.
	defaultOpenAIScheme = "https"

	// defaultOpenAIHost is the default host for the OpenAI Chat Completions API.
	defaultOpenAIHost = "api.openai.com"

	// defaultOpenAIPathPrefix is the default path prefix for OpenAI (empty: the
	// endpoint path is appended by the generator).
	defaultOpenAIPathPrefix = ""

	// defaultOpenAIModel is the default model used when none is specified in the URL.
	defaultOpenAIModel = "gpt-4o-mini"

	// envOpenAIAPIKey is the environment variable for the OpenAI API key.
	envOpenAIAPIKey = "OPENAI_API_KEY"
)

// OpenAIOpener implements the Opener interface for the OpenAI Chat Completions
// API. It supports the "openai" URL scheme.
//
// URL format: openai://[userinfo@]{host}[:port]/{path_prefix...}/{model}
//
// The scheme identifies the OpenAI provider. The netloc and path prefix (minus
// the last segment) form the base URL. The last path segment is the model name.
// The API key is read exclusively from the OPENAI_API_KEY environment variable.
//
// Examples:
//   - openai://                       (defaults: api.openai.com, gpt-4o-mini)
//   - openai:///gpt-4o                (default host, specific model)
//   - openai://api.openai.com/gpt-4o  (fully specified)
type OpenAIOpener struct{}

// Id returns the unique identifier for the OpenAI opener.
func (o *OpenAIOpener) Id() string { return "openai" }

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "openai" scheme.
func (o *OpenAIOpener) CanOpen(u *url.URL) bool { return u.Scheme == "openai" }

// Open creates an OpenAI generator client using the provided URL.
// The API key is read from the OPENAI_API_KEY environment variable.
func (o *OpenAIOpener) Open(_ context.Context, u *url.URL) (Generator, error) {
	if u == nil {
		return nil, errors.New("generators: URL cannot be nil for OpenAIOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("generators: scheme %q not supported by OpenAIOpener (expected openai)", u.Scheme)
	}

	cfg := parseProviderURL(u, defaultOpenAIScheme, defaultOpenAIHost, defaultOpenAIPathPrefix, defaultOpenAIModel)

	return &OpenAIGenerator{
		httpClient: &http.Client{},
		apiKey:     lookupOpenAIAPIKey(),
		model:      cfg.model,
		baseURL:    cfg.baseURL,
	}, nil
}

func init() {
	RegisterOpener(&OpenAIOpener{})
}

// lookupOpenAIAPIKey reads the API key from the OPENAI_API_KEY environment variable.
func lookupOpenAIAPIKey() string {
	return os.Getenv(envOpenAIAPIKey)
}

// --- OpenAIGenerator ---

// OpenAIGenerator implements the Generator interface for OpenAI using the
// Chat Completions API (/v1/chat/completions) via net/http.
type OpenAIGenerator struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
}

// Generate produces a text completion for the given prompt using OpenAI Chat
// Completions.
func (g *OpenAIGenerator) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	cfg := newConfig(opts)
	cfg.Model = g.resolveModel(cfg)

	reqBody := buildChatCompletionRequest(cfg, prompt)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: openai marshal request: %w", err)
	}

	endpoint := g.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: openai create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: openai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("generators: openai read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generators: openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parseChatCompletionResponse(respBody, cfg.Model)
}

// Stream produces a streaming text completion for the given prompt using OpenAI
// Chat Completions, returning a channel fed by the SSE response.
func (g *OpenAIGenerator) Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error) {
	cfg := newConfig(opts)
	cfg.Model = g.resolveModel(cfg)

	reqBody := buildChatCompletionRequest(cfg, prompt)
	reqBody.Stream = true
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: openai marshal request: %w", err)
	}

	endpoint := g.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: openai create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: openai stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("generators: openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		consumeChatCompletionSSE(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// Close releases the resources held by the OpenAI generator.
func (g *OpenAIGenerator) Close() error { return nil }

// resolveModel returns the model from the config if set, otherwise the default.
func (g *OpenAIGenerator) resolveModel(cfg *Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return g.model
}
