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
)

const (
	// defaultOllamaScheme is the HTTP scheme for the Ollama API.
	defaultOllamaScheme = "http"

	// defaultOllamaHost is the default host for the Ollama REST API.
	defaultOllamaHost = "localhost:11434"

	// defaultOllamaPathPrefix is the default path prefix (empty for Ollama).
	defaultOllamaPathPrefix = ""

	// defaultOllamaModel is the default model used when none is specified in the URL.
	defaultOllamaModel = "llama3.2"
)

// OllamaOpener implements the Opener interface for a local Ollama server.
// It supports the "ollama" URL scheme and uses the Ollama REST API directly.
//
// URL format: ollama://[userinfo@]{host}[:port]/{path_prefix...}/{model}
//
// The scheme identifies the Ollama provider. The netloc and path prefix (minus
// the last segment) form the base URL. The last path segment is the model name.
//
// Examples:
//   - ollama://                                      (defaults: localhost:11434, llama3.2)
//   - ollama://localhost:11434/llama3.2              (explicit host and model)
//   - ollama://localhost:11434/gemma3                (host with alternative model)
//   - ollama://user:pass@my-server:8080/qwen3        (remote with auth)
type OllamaOpener struct{}

// Id returns the unique identifier for the Ollama opener.
func (o *OllamaOpener) Id() string {
	return "ollama"
}

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "ollama" scheme.
func (o *OllamaOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "ollama"
}

// Open creates an Ollama generator client using the provided URL.
func (o *OllamaOpener) Open(_ context.Context, u *url.URL) (Generator, error) {
	if u == nil {
		return nil, errors.New("generators: URL cannot be nil for OllamaOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("generators: scheme %q not supported by OllamaOpener (expected ollama)", u.Scheme)
	}

	cfg := parseProviderURL(u, defaultOllamaScheme, defaultOllamaHost, defaultOllamaPathPrefix, defaultOllamaModel)

	return &OllamaGenerator{
		httpClient: &http.Client{},
		baseURL:    cfg.baseURL,
		model:      cfg.model,
	}, nil
}

func init() {
	RegisterOpener(&OllamaOpener{})
}

// --- Internal JSON types for the Ollama REST API ---

type ollamaRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	Stream  bool          `json:"stream"`
	System  string        `json:"system,omitempty"`
	Options ollamaOptions `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature *float32 `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	TopP        *float32 `json:"top_p,omitempty"`
	TopK        int      `json:"top_k,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ollamaResponse struct {
	Model           string `json:"model"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason,omitempty"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
}

// --- OllamaGenerator ---

// OllamaGenerator implements the Generator interface for Ollama
// using the REST API directly via net/http.
type OllamaGenerator struct {
	httpClient *http.Client
	baseURL    string
	model      string
}

// Generate produces a text completion for the given prompt using the Ollama REST API.
func (g *OllamaGenerator) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	cfg := newConfig(opts)
	reqBody := g.buildRequest(cfg, prompt, false)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: ollama marshal request: %w", err)
	}

	model := g.resolveModel(cfg)
	endpoint := g.baseURL + "/api/generate"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: ollama create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("generators: ollama API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ollResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollResp); err != nil {
		return nil, fmt.Errorf("generators: ollama decode response: %w", err)
	}

	return g.mapResponse(&ollResp, model), nil
}

// Stream produces a streaming text completion for the given prompt using the Ollama REST API.
// Returns a read-only channel that yields response chunks as NDJSON lines arrive.
func (g *OllamaGenerator) Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error) {
	cfg := newConfig(opts)
	reqBody := g.buildRequest(cfg, prompt, true)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: ollama marshal request: %w", err)
	}

	endpoint := g.baseURL + "/api/generate"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: ollama create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: ollama stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("generators: ollama API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)
		defer resp.Body.Close()
		g.consumeNDJSON(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// Close releases the resources held by the Ollama generator.
func (g *OllamaGenerator) Close() error {
	return nil
}

// resolveModel returns the model from the config if set, otherwise the default.
func (g *OllamaGenerator) resolveModel(cfg *Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return g.model
}

// buildRequest converts a Config and prompt into an Ollama API request.
func (g *OllamaGenerator) buildRequest(cfg *Config, prompt string, stream bool) ollamaRequest {
	req := ollamaRequest{
		Model:  g.resolveModel(cfg),
		Prompt: prompt,
		Stream: stream,
	}

	opts := ollamaOptions{}
	hasOpts := false

	if cfg.Temperature != 0 {
		opts.Temperature = ptrFloat32(cfg.Temperature)
		hasOpts = true
	}
	if cfg.MaxOutputTokens != 0 {
		opts.NumPredict = cfg.MaxOutputTokens
		hasOpts = true
	}
	if cfg.TopP != 0 {
		opts.TopP = ptrFloat32(cfg.TopP)
		hasOpts = true
	}
	if cfg.TopK != 0 {
		opts.TopK = int(cfg.TopK)
		hasOpts = true
	}
	if len(cfg.StopSequences) > 0 {
		opts.Stop = cfg.StopSequences
		hasOpts = true
	}
	if hasOpts {
		req.Options = opts
	}

	if cfg.SystemInstruction != "" {
		req.System = cfg.SystemInstruction
	}

	return req
}

// mapResponse converts an ollamaResponse into a generators.Response.
func (g *OllamaGenerator) mapResponse(resp *ollamaResponse, model string) *Response {
	out := &Response{
		Model:        model,
		Text:         resp.Response,
		FinishReason: resp.DoneReason,
	}

	if resp.PromptEvalCount > 0 || resp.EvalCount > 0 {
		out.Usage = Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		}
	}

	return out
}

// consumeNDJSON reads newline-delimited JSON from the response body and sends
// parsed chunks on the channel.
func (g *OllamaGenerator) consumeNDJSON(ctx context.Context, body io.Reader, ch chan<- StreamChunk) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			ch <- StreamChunk{Error: ctx.Err()}
			return
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollResp ollamaResponse
		if err := json.Unmarshal([]byte(line), &ollResp); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("generators: ollama NDJSON unmarshal: %w", err)}
			return
		}

		if ollResp.Response != "" {
			ch <- StreamChunk{Text: ollResp.Response}
		}

		if ollResp.Done {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("generators: ollama NDJSON read: %w", err)}
	}
}
