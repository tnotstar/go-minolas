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
	// defaultAnthropicScheme is the HTTP scheme for the Anthropic API.
	defaultAnthropicScheme = "https"

	// defaultAnthropicHost is the default host for the Anthropic Messages API.
	defaultAnthropicHost = "api.anthropic.com"

	// defaultAnthropicPathPrefix is the default path prefix (empty: the
	// /v1/messages endpoint is appended by the generator).
	defaultAnthropicPathPrefix = ""

	// defaultAnthropicModel is the default model used when none is specified.
	defaultAnthropicModel = "claude-3-5-sonnet-latest"

	// defaultAnthropicMaxTokens is the default max_tokens value. Anthropic
	// mandates max_tokens on every request, so we supply a sane default when
	// WithMaxOutputTokens is not set (see design.md D7).
	defaultAnthropicMaxTokens = 1024

	// envAnthropicAPIKey is the environment variable for the Anthropic API key.
	envAnthropicAPIKey = "ANTHROPIC_API_KEY"

	// anthropicVersion is the pinned Messages API version sent as a header.
	// Update when Anthropic releases a stable newer version.
	anthropicVersion = "2023-06-01"
)

// AnthropicOpener implements the Opener interface for the Anthropic Messages
// API. It supports the "anthropic" URL scheme.
//
// URL format: anthropic://[userinfo@]{host}[:port]/{path_prefix...}/{model}
//
// The API key is read exclusively from the ANTHROPIC_API_KEY environment
// variable.
//
// Examples:
//   - anthropic://                                (defaults)
//   - anthropic:///claude-3.5-sonnet              (default host, specific model)
//   - anthropic://api.anthropic.com/claude-3-opus (fully specified)
type AnthropicOpener struct{}

// Id returns the unique identifier for the Anthropic opener.
func (o *AnthropicOpener) Id() string { return "anthropic" }

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "anthropic" scheme.
func (o *AnthropicOpener) CanOpen(u *url.URL) bool { return u.Scheme == "anthropic" }

// Open creates an Anthropic generator client using the provided URL.
// The API key is read from the ANTHROPIC_API_KEY environment variable.
func (o *AnthropicOpener) Open(_ context.Context, u *url.URL) (Generator, error) {
	if u == nil {
		return nil, errors.New("generators: URL cannot be nil for AnthropicOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("generators: scheme %q not supported by AnthropicOpener (expected anthropic)", u.Scheme)
	}

	cfg := parseProviderURL(u, defaultAnthropicScheme, defaultAnthropicHost, defaultAnthropicPathPrefix, defaultAnthropicModel)

	return &AnthropicGenerator{
		httpClient: &http.Client{},
		apiKey:     lookupAnthropicAPIKey(),
		model:      cfg.model,
		baseURL:    cfg.baseURL,
	}, nil
}

func init() {
	RegisterOpener(&AnthropicOpener{})
}

// lookupAnthropicAPIKey reads the API key from the ANTHROPIC_API_KEY env var.
func lookupAnthropicAPIKey() string {
	return os.Getenv(envAnthropicAPIKey)
}

// --- Internal JSON types for the Anthropic Messages API ---

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature *float32           `json:"temperature,omitempty"`
	TopP        *float32           `json:"top_p,omitempty"`
	TopK        *int               `json:"top_k,omitempty"`
	Stop        []string           `json:"stop_sequences,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Stop    string             `json:"stop_reason"`
	Usage   *anthropicUsage    `json:"usage"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicStreamEvent is the payload carried by SSE data lines during streaming.
type anthropicStreamEvent struct {
	Type  string                `json:"type"`
	Delta *anthropicStreamDelta `json:"delta,omitempty"`
}

type anthropicStreamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// --- AnthropicGenerator ---

// AnthropicGenerator implements the Generator interface for Anthropic using the
// Messages API (/v1/messages) via net/http.
type AnthropicGenerator struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
}

// Generate produces a text completion for the given prompt using Anthropic.
func (g *AnthropicGenerator) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	cfg := newConfig(opts)
	cfg.Model = g.resolveModel(cfg)

	reqBody := g.buildRequest(cfg, prompt)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic marshal request: %w", err)
	}

	endpoint := g.baseURL + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic create request: %w", err)
	}
	g.setHeaders(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generators: anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return g.parseResponse(respBody, cfg.Model)
}

// Stream produces a streaming text completion for the given prompt using
// Anthropic, returning a channel fed by the typed SSE response.
func (g *AnthropicGenerator) Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error) {
	cfg := newConfig(opts)
	cfg.Model = g.resolveModel(cfg)

	reqBody := g.buildRequest(cfg, prompt)
	reqBody.Stream = true
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic marshal request: %w", err)
	}

	endpoint := g.baseURL + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic create request: %w", err)
	}
	g.setHeaders(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: anthropic stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("generators: anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		consumeAnthropicSSE(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// Close releases the resources held by the Anthropic generator.
func (g *AnthropicGenerator) Close() error { return nil }

// setHeaders applies the authentication and version headers required by the
// Anthropic Messages API.
func (g *AnthropicGenerator) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
}

// resolveModel returns the model from the config if set, otherwise the default.
func (g *AnthropicGenerator) resolveModel(cfg *Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return g.model
}

// buildRequest converts a Config and prompt into an Anthropic Messages request.
func (g *AnthropicGenerator) buildRequest(cfg *Config, prompt string) anthropicRequest {
	req := anthropicRequest{
		Model:     g.resolveModel(cfg),
		MaxTokens: defaultAnthropicMaxTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}

	if cfg.MaxOutputTokens != 0 {
		req.MaxTokens = cfg.MaxOutputTokens
	}
	if cfg.Temperature != 0 {
		req.Temperature = ptrFloat32(cfg.Temperature)
	}
	if cfg.TopP != 0 {
		req.TopP = ptrFloat32(cfg.TopP)
	}
	if cfg.TopK != 0 {
		k := int(cfg.TopK)
		req.TopK = &k
	}
	if len(cfg.StopSequences) > 0 {
		req.Stop = cfg.StopSequences
	}
	if cfg.SystemInstruction != "" {
		req.System = cfg.SystemInstruction
	}

	return req
}

// parseResponse decodes an Anthropic Messages response body into a Response.
func (g *AnthropicGenerator) parseResponse(body []byte, model string) (*Response, error) {
	var aResp anthropicResponse
	if err := json.Unmarshal(body, &aResp); err != nil {
		return nil, fmt.Errorf("generators: anthropic decode response: %w", err)
	}

	out := &Response{Model: model, FinishReason: aResp.Stop}
	var texts []string
	for _, c := range aResp.Content {
		if c.Type == "text" && c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	out.Text = strings.Join(texts, "")

	if aResp.Usage != nil {
		out.Usage = Usage{
			PromptTokens:     aResp.Usage.InputTokens,
			CompletionTokens: aResp.Usage.OutputTokens,
			TotalTokens:      aResp.Usage.InputTokens + aResp.Usage.OutputTokens,
		}
	}
	return out, nil
}

// consumeAnthropicSSE reads typed Server-Sent Events from the response body and
// sends text deltas on the channel. Content arrives on content_block_delta
// events; the stream terminates at message_stop.
func consumeAnthropicSSE(ctx context.Context, body io.Reader, ch chan<- StreamChunk) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			ch <- StreamChunk{Error: ctx.Err()}
			return
		}

		line := scanner.Text()

		// Event type lines start with "event: ".
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			// The next non-empty line is the data payload. We peek by scanning
			// the next line; if it isn't a data line, skip.
			if !scanner.Scan() {
				break
			}
			dataLine := scanner.Text()
			if !strings.HasPrefix(dataLine, "data: ") {
				continue
			}

			if eventType == "message_stop" {
				return
			}
			if eventType != "content_block_delta" {
				continue
			}

			data := strings.TrimPrefix(dataLine, "data: ")
			var evt anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				ch <- StreamChunk{Error: fmt.Errorf("generators: anthropic SSE unmarshal: %w", err)}
				return
			}
			if evt.Delta != nil && evt.Delta.Text != "" {
				ch <- StreamChunk{Text: evt.Delta.Text}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("generators: anthropic SSE read: %w", err)}
	}
}
