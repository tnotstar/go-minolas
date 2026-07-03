package generators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// --- Opener tests ---

func TestAnthropicOpener_Id(t *testing.T) {
	op := &AnthropicOpener{}
	if got := op.Id(); got != "anthropic" {
		t.Errorf("AnthropicOpener.Id() = %q, want %q", got, "anthropic")
	}
}

func TestAnthropicOpener_CanOpen(t *testing.T) {
	op := &AnthropicOpener{}

	testCases := []struct {
		name   string
		rawurl string
		want   bool
	}{
		{name: "anthropic scheme", rawurl: "anthropic://", want: true},
		{name: "anthropic with model", rawurl: "anthropic:///claude-3.5-sonnet", want: true},
		{name: "openai scheme", rawurl: "openai://", want: false},
		{name: "https scheme", rawurl: "https://api.anthropic.com", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawurl)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.rawurl, err)
			}
			if got := op.CanOpen(u); got != tc.want {
				t.Errorf("CanOpen(%q) = %v, want %v", tc.rawurl, got, tc.want)
			}
		})
	}
}

func TestAnthropicOpener_Open_NilURL(t *testing.T) {
	op := &AnthropicOpener{}
	if _, err := op.Open(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil URL, got nil")
	}
}

func TestAnthropicOpener_Open_InvalidScheme(t *testing.T) {
	op := &AnthropicOpener{}
	u, _ := url.Parse("openai://")
	if _, err := op.Open(context.Background(), u); err == nil {
		t.Fatal("expected error for invalid scheme, got nil")
	}
}

func TestAnthropicOpener_Open_Defaults(t *testing.T) {
	op := &AnthropicOpener{}
	u, _ := url.Parse("anthropic://")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g, ok := gen.(*AnthropicGenerator)
	if !ok {
		t.Fatal("expected *AnthropicGenerator")
	}
	if g.baseURL != "https://api.anthropic.com" {
		t.Errorf("baseURL = %q, want https://api.anthropic.com", g.baseURL)
	}
	if g.model != defaultAnthropicModel {
		t.Errorf("model = %q, want %q", g.model, defaultAnthropicModel)
	}
}

func TestAnthropicOpener_Open_ModelOnly(t *testing.T) {
	op := &AnthropicOpener{}
	u, _ := url.Parse("anthropic:///claude-3.5-sonnet")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := gen.(*AnthropicGenerator)
	if g.model != "claude-3.5-sonnet" {
		t.Errorf("model = %q, want claude-3.5-sonnet", g.model)
	}
}

// --- API key lookup tests ---

func TestLookupAnthropicAPIKey(t *testing.T) {
	orig := os.Getenv(envAnthropicAPIKey)
	t.Cleanup(func() { os.Setenv(envAnthropicAPIKey, orig) })

	testCases := []struct {
		name    string
		envSet  map[string]string
		wantKey string
	}{
		{name: "ANTHROPIC_API_KEY set", envSet: map[string]string{envAnthropicAPIKey: "sk-ant"}, wantKey: "sk-ant"},
		{name: "no key", envSet: map[string]string{}, wantKey: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(envAnthropicAPIKey)
			for k, v := range tc.envSet {
				os.Setenv(k, v)
			}
			if got := lookupAnthropicAPIKey(); got != tc.wantKey {
				t.Errorf("lookupAnthropicAPIKey() = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

// --- request building tests ---

func TestAnthropicBuildRequest(t *testing.T) {
	g := &AnthropicGenerator{}

	t.Run("minimal config defaults max_tokens", func(t *testing.T) {
		req := g.buildRequest(&Config{}, "hello")
		if req.MaxTokens != defaultAnthropicMaxTokens {
			t.Errorf("MaxTokens = %d, want %d", req.MaxTokens, defaultAnthropicMaxTokens)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
			t.Errorf("expected single user message, got %+v", req.Messages)
		}
		if req.System != "" {
			t.Errorf("expected empty system for minimal config")
		}
	})

	t.Run("max_tokens overridable", func(t *testing.T) {
		req := g.buildRequest(&Config{MaxOutputTokens: 512}, "hello")
		if req.MaxTokens != 512 {
			t.Errorf("MaxTokens = %d, want 512", req.MaxTokens)
		}
	})

	t.Run("system instruction goes to top-level system field", func(t *testing.T) {
		req := g.buildRequest(&Config{SystemInstruction: "be brief"}, "hi")
		if req.System != "be brief" {
			t.Errorf("System = %q, want be brief", req.System)
		}
	})

	t.Run("options map to wire fields", func(t *testing.T) {
		req := g.buildRequest(&Config{Temperature: 0.5, TopP: 0.9, TopK: 40, StopSequences: []string{"END"}}, "p")
		if req.Temperature == nil || *req.Temperature != 0.5 {
			t.Errorf("Temperature = %v, want 0.5", req.Temperature)
		}
		if req.TopP == nil || *req.TopP != 0.9 {
			t.Errorf("TopP = %v, want 0.9", req.TopP)
		}
		if req.TopK == nil || *req.TopK != 40 {
			t.Errorf("TopK = %v, want 40", req.TopK)
		}
		if len(req.Stop) != 1 || req.Stop[0] != "END" {
			t.Errorf("Stop = %v, want [END]", req.Stop)
		}
	})
}

func TestAnthropicParseResponse(t *testing.T) {
	g := &AnthropicGenerator{}

	t.Run("maps content stop_reason and usage", func(t *testing.T) {
		body := []byte(`{
			"content":[{"type":"text","text":"hi"}],
			"stop_reason":"end_turn",
			"usage":{"input_tokens":2,"output_tokens":1}
		}`)
		resp, err := g.parseResponse(body, "claude")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "hi" {
			t.Errorf("Text = %q, want hi", resp.Text)
		}
		if resp.FinishReason != "end_turn" {
			t.Errorf("FinishReason = %q, want end_turn", resp.FinishReason)
		}
		if resp.Usage.PromptTokens != 2 || resp.Usage.CompletionTokens != 1 || resp.Usage.TotalTokens != 3 {
			t.Errorf("Usage = %+v, want 2/1/3", resp.Usage)
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		_, err := g.parseResponse([]byte("{bad"), "m")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

// --- httptest.Server integration ---

func TestAnthropicGenerate_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("anthropic-version") != anthropicVersion {
			http.Error(w, "bad version", http.StatusBadRequest)
			return
		}
		resp := anthropicResponse{
			Content: []anthropicContent{{Type: "text", Text: "test response"}},
			Stop:    "end_turn",
			Usage:   &anthropicUsage{InputTokens: 5, OutputTokens: 3},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &AnthropicGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-ant-test",
		model:      defaultAnthropicModel,
		baseURL:    server.URL,
	}

	resp, err := gen.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "test response" {
		t.Errorf("Text = %q, want test response", resp.Text)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("TotalTokens = %d, want 8", resp.Usage.TotalTokens)
	}
}

func TestAnthropicGenerate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	gen := &AnthropicGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-ant-test",
		model:      defaultAnthropicModel,
		baseURL:    server.URL,
	}

	if _, err := gen.Generate(context.Background(), "hello"); err == nil {
		t.Fatal("expected error for 429, got nil")
	}
}

func TestAnthropicStream_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{"Hello ", "world"}
		for _, chunk := range chunks {
			payload := fmt.Sprintf(`{"type":"content_block_delta","delta":{"type":"text_delta","text":%q}}`, chunk)
			fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", payload)
		}
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer server.Close()

	gen := &AnthropicGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-ant-test",
		model:      defaultAnthropicModel,
		baseURL:    server.URL,
	}

	ch, err := gen.Stream(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	var collected string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("Stream chunk error: %v", chunk.Error)
		}
		collected += chunk.Text
	}
	if collected != "Hello world" {
		t.Errorf("collected = %q, want %q", collected, "Hello world")
	}
}

func TestAnthropicGenerator_Close(t *testing.T) {
	gen := &AnthropicGenerator{}
	if err := gen.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestAnthropicConsumeSSE(t *testing.T) {
	t.Run("emits deltas and stops at message_stop", func(t *testing.T) {
		body := strings.NewReader(
			"event: ping\ndata: {\"type\":\"ping\"}\n\n" +
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n" +
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n" +
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ignored\"}}\n\n",
		)
		ch := make(chan StreamChunk, 4)
		consumeAnthropicSSE(context.Background(), body, ch)
		close(ch)
		var got string
		for c := range ch {
			got += c.Text
		}
		if got != "Hi" {
			t.Errorf("collected = %q, want Hi", got)
		}
	})

	t.Run("propagates malformed data as error", func(t *testing.T) {
		ch := make(chan StreamChunk, 2)
		consumeAnthropicSSE(context.Background(), strings.NewReader("event: content_block_delta\ndata: not-json\n\n"), ch)
		close(ch)
		var sawErr bool
		for c := range ch {
			if c.Error != nil {
				sawErr = true
			}
		}
		if !sawErr {
			t.Error("expected an error chunk for malformed data")
		}
	})
}
