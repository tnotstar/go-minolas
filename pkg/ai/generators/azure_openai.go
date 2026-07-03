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
	// defaultAzureOpenAIScheme is the HTTP scheme for Azure OpenAI.
	defaultAzureOpenAIScheme = "https"

	// defaultAzureOpenAIAPIVersion is the pinned Azure OpenAI Chat Completions
	// API version, sent as the api-version query parameter.
	// Update when Azure deprecates this version.
	defaultAzureOpenAIAPIVersion = "2024-10-21"

	// envAzureOpenAIAPIKey is the environment variable for the Azure OpenAI key.
	envAzureOpenAIAPIKey = "AZURE_OPENAI_API_KEY"
)

// AzureOpenAIOpener implements the Opener interface for Azure OpenAI. It
// supports the "azure-openai" URL scheme.
//
// URL format: azure-openai://{resource}.cognitiveservices.azure.com/{deployment}
//
// Azure OpenAI is deployment-scoped rather than model-scoped: the host names the
// Azure resource and the last path segment names the deployment. Both are
// REQUIRED. Authentication uses the api-key header with the
// AZURE_OPENAI_API_KEY environment variable.
//
// Examples:
//   - azure-openai://my-resource.cognitiveservices.azure.com/my-deployment
//   - azure-openai://my-resource.cognitiveservices.azure.com/gpt-4o-deploy
type AzureOpenAIOpener struct{}

// Id returns the unique identifier for the Azure OpenAI opener.
func (o *AzureOpenAIOpener) Id() string { return "azure-openai" }

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "azure-openai" scheme.
func (o *AzureOpenAIOpener) CanOpen(u *url.URL) bool { return u.Scheme == "azure-openai" }

// Open creates an Azure OpenAI generator client using the provided URL.
// Both the host (resource) and the last path segment (deployment) are required.
func (o *AzureOpenAIOpener) Open(_ context.Context, u *url.URL) (Generator, error) {
	if u == nil {
		return nil, errors.New("generators: URL cannot be nil for AzureOpenAIOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("generators: scheme %q not supported by AzureOpenAIOpener (expected azure-openai)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("generators: azure-openai URL must include a resource host (e.g. azure-openai://{resource}.cognitiveservices.azure.com/{deployment})")
	}

	_, deployment := splitLastSegment(u.EscapedPath())
	if deployment == "" {
		return nil, errors.New("generators: azure-openai URL must include a deployment as the last path segment (e.g. azure-openai://host/{deployment})")
	}

	base := &url.URL{
		Scheme: defaultAzureOpenAIScheme,
		Host:   u.Host,
	}
	if u.User != nil {
		base.User = u.User
	}

	return &AzureOpenAIGenerator{
		httpClient: &http.Client{},
		apiKey:     lookupAzureOpenAIAPIKey(),
		baseURL:    base.String(),
		deployment: deployment,
		apiVersion: defaultAzureOpenAIAPIVersion,
	}, nil
}

func init() {
	RegisterOpener(&AzureOpenAIOpener{})
}

// lookupAzureOpenAIAPIKey reads the key from AZURE_OPENAI_API_KEY.
func lookupAzureOpenAIAPIKey() string {
	return os.Getenv(envAzureOpenAIAPIKey)
}

// --- AzureOpenAIGenerator ---

// AzureOpenAIGenerator implements the Generator interface for Azure OpenAI,
// sharing the Chat Completions wire-format helpers with the openai/compat
// providers but differing in endpoint construction and authentication header.
type AzureOpenAIGenerator struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	deployment string
	apiVersion string
}

// endpoint builds the deployment-scoped Chat Completions URL with api-version.
func (g *AzureOpenAIGenerator) endpoint() string {
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		g.baseURL, g.deployment, g.apiVersion)
}

// Generate produces a text completion for the given prompt via Azure OpenAI.
func (g *AzureOpenAIGenerator) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	cfg := newConfig(opts)
	// Azure is deployment-scoped: do not override the model in the request body
	// (the deployment already pins it), but keep Config.Model for the response.
	reqBody := buildChatCompletionRequest(cfg, prompt)
	reqBody.Model = "" // Azure ignores model in the body; deployment selects it.

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generators: azure-openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parseChatCompletionResponse(respBody, cfg.Model)
}

// Stream produces a streaming text completion via Azure OpenAI using SSE.
func (g *AzureOpenAIGenerator) Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error) {
	cfg := newConfig(opts)
	reqBody := buildChatCompletionRequest(cfg, prompt)
	reqBody.Model = ""
	reqBody.Stream = true

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generators: azure-openai stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("generators: azure-openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		consumeChatCompletionSSE(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// Close releases the resources held by the Azure OpenAI generator.
func (g *AzureOpenAIGenerator) Close() error { return nil }
