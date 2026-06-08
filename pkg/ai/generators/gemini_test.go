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

func TestGeminiOpener_Id(t *testing.T) {
	op := &GeminiOpener{}
	if got := op.Id(); got != "gemini" {
		t.Errorf("GeminiOpener.Id() = %q, want %q", got, "gemini")
	}
}

func TestGeminiOpener_CanOpen(t *testing.T) {
	op := &GeminiOpener{}

	testCases := []struct {
		name   string
		rawurl string
		want   bool
	}{
		{name: "gemini scheme", rawurl: "gemini://", want: true},
		{name: "gemini with host", rawurl: "gemini://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash", want: true},
		{name: "ollama scheme", rawurl: "ollama://localhost", want: false},
		{name: "http scheme", rawurl: "http://example.com", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawurl)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", tc.rawurl, err)
			}
			if got := op.CanOpen(u); got != tc.want {
				t.Errorf("CanOpen(%q) = %v, want %v", tc.rawurl, got, tc.want)
			}
		})
	}
}

func TestGeminiOpener_Open_NilURL(t *testing.T) {
	op := &GeminiOpener{}
	_, err := op.Open(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil URL, got nil")
	}
}

func TestGeminiOpener_Open_InvalidScheme(t *testing.T) {
	op := &GeminiOpener{}
	u, _ := url.Parse("ollama://localhost")
	_, err := op.Open(context.Background(), u)
	if err == nil {
		t.Fatal("expected error for invalid scheme, got nil")
	}
}

func TestGeminiOpener_Open_Defaults(t *testing.T) {
	op := &GeminiOpener{}
	u, _ := url.Parse("gemini://")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g, ok := gen.(*GeminiGenerator)
	if !ok {
		t.Fatal("expected *GeminiGenerator")
	}
	if g.baseURL != "https://generativelanguage.googleapis.com/v1beta/models" {
		t.Errorf("baseURL = %q, want default Gemini base URL", g.baseURL)
	}
	if g.model != defaultGeminiModel {
		t.Errorf("model = %q, want %q", g.model, defaultGeminiModel)
	}
}

func TestGeminiOpener_Open_ModelOnly(t *testing.T) {
	op := &GeminiOpener{}
	u, _ := url.Parse("gemini:///gemini-2.0-pro")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := gen.(*GeminiGenerator)
	if g.model != "gemini-2.0-pro" {
		t.Errorf("model = %q, want %q", g.model, "gemini-2.0-pro")
	}
	if g.baseURL != "https://generativelanguage.googleapis.com/v1beta/models" {
		t.Errorf("baseURL = %q, want default with path prefix", g.baseURL)
	}
}

func TestGeminiOpener_Open_FullySpecified(t *testing.T) {
	op := &GeminiOpener{}
	u, _ := url.Parse("gemini://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := gen.(*GeminiGenerator)
	if g.baseURL != "https://generativelanguage.googleapis.com/v1beta/models" {
		t.Errorf("baseURL = %q, want %q", g.baseURL, "https://generativelanguage.googleapis.com/v1beta/models")
	}
	if g.model != "gemini-2.0-flash" {
		t.Errorf("model = %q, want %q", g.model, "gemini-2.0-flash")
	}
}

// --- API key lookup tests ---

func TestLookupGeminiAPIKey(t *testing.T) {
	origGemini := os.Getenv(envGeminiAPIKey)
	origGoogle := os.Getenv(envGoogleAPIKey)
	t.Cleanup(func() {
		os.Setenv(envGeminiAPIKey, origGemini)
		os.Setenv(envGoogleAPIKey, origGoogle)
	})

	testCases := []struct {
		name    string
		envSet  map[string]string
		wantKey string
	}{
		{name: "GEMINI_API_KEY", envSet: map[string]string{envGeminiAPIKey: "gemini-key"}, wantKey: "gemini-key"},
		{name: "GOOGLE_API_KEY fallback", envSet: map[string]string{envGoogleAPIKey: "google-key"}, wantKey: "google-key"},
		{name: "GEMINI takes precedence", envSet: map[string]string{envGeminiAPIKey: "gemini", envGoogleAPIKey: "google"}, wantKey: "gemini"},
		{name: "no key", envSet: map[string]string{}, wantKey: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(envGeminiAPIKey)
			os.Unsetenv(envGoogleAPIKey)
			for k, v := range tc.envSet {
				os.Setenv(k, v)
			}
			got := lookupGeminiAPIKey()
			if got != tc.wantKey {
				t.Errorf("lookupGeminiAPIKey() = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

// --- Request/response mapping tests ---

func TestGeminiBuildRequestBody(t *testing.T) {
	g := &GeminiGenerator{}

	t.Run("minimal config produces prompt only", func(t *testing.T) {
		cfg := &Config{}
		req := g.buildRequestBody(cfg, "hello")
		if len(req.Contents) != 1 || req.Contents[0].Parts[0].Text != "hello" {
			t.Error("prompt not set correctly")
		}
		if req.GenerationConfig != nil {
			t.Error("expected nil GenerationConfig for minimal config")
		}
		if req.SystemInstruction != nil {
			t.Error("expected nil SystemInstruction for minimal config")
		}
	})

	t.Run("full config sets all fields", func(t *testing.T) {
		cfg := &Config{
			Temperature:       0.7,
			MaxOutputTokens:   256,
			TopP:              0.9,
			TopK:              40,
			SystemInstruction: "You are a helpful assistant.",
			StopSequences:     []string{"END", "STOP"},
		}
		req := g.buildRequestBody(cfg, "test prompt")

		if req.GenerationConfig == nil {
			t.Fatal("GenerationConfig should not be nil")
		}
		if *req.GenerationConfig.Temperature != 0.7 {
			t.Errorf("Temperature = %v, want 0.7", *req.GenerationConfig.Temperature)
		}
		if req.GenerationConfig.MaxOutputTokens != 256 {
			t.Errorf("MaxOutputTokens = %d, want 256", req.GenerationConfig.MaxOutputTokens)
		}
		if req.SystemInstruction == nil || req.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
			t.Error("SystemInstruction not set correctly")
		}
	})

	t.Run("config serializes to valid JSON", func(t *testing.T) {
		cfg := &Config{Temperature: 0.5, MaxOutputTokens: 100}
		req := g.buildRequestBody(cfg, "test")
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(data), `"temperature"`) {
			t.Error("JSON should contain temperature")
		}
	})
}

func TestGeminiParseResponse(t *testing.T) {
	g := &GeminiGenerator{}

	t.Run("nil response returns empty", func(t *testing.T) {
		resp := g.parseResponse(nil, "model")
		if resp.Text != "" {
			t.Error("expected empty text for nil response")
		}
	})

	t.Run("full response maps correctly", func(t *testing.T) {
		gemResp := &geminiResponse{
			Candidates: []struct {
				Content      geminiContent `json:"content"`
				FinishReason string        `json:"finishReason"`
			}{
				{
					Content:      geminiContent{Parts: []geminiPart{{Text: "Hello world"}}},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			}{PromptTokenCount: 10, CandidatesTokenCount: 5, TotalTokenCount: 15},
		}

		resp := g.parseResponse(gemResp, "gemini-2.0-flash")
		if resp.Text != "Hello world" {
			t.Errorf("Text = %q, want %q", resp.Text, "Hello world")
		}
		if resp.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
		}
	})

	t.Run("multi-part content is concatenated", func(t *testing.T) {
		gemResp := &geminiResponse{
			Candidates: []struct {
				Content      geminiContent `json:"content"`
				FinishReason string        `json:"finishReason"`
			}{
				{Content: geminiContent{Parts: []geminiPart{{Text: "Hello "}, {Text: "world"}}}},
			},
		}
		resp := g.parseResponse(gemResp, "model")
		if resp.Text != "Hello world" {
			t.Errorf("Text = %q, want %q", resp.Text, "Hello world")
		}
	})
}

func TestGeminiResolveModel(t *testing.T) {
	g := &GeminiGenerator{model: "default-model"}

	if got := g.resolveModel(&Config{Model: "override"}); got != "override" {
		t.Errorf("with override = %q, want %q", got, "override")
	}
	if got := g.resolveModel(&Config{Model: ""}); got != "default-model" {
		t.Errorf("without override = %q, want %q", got, "default-model")
	}
}

// --- httptest.Server integration ---

func TestGeminiGenerate_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		resp := geminiResponse{
			Candidates: []struct {
				Content      geminiContent `json:"content"`
				FinishReason string        `json:"finishReason"`
			}{
				{Content: geminiContent{Parts: []geminiPart{{Text: "test response"}}}, FinishReason: "STOP"},
			},
			UsageMetadata: &struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			}{PromptTokenCount: 5, CandidatesTokenCount: 3, TotalTokenCount: 8},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &GeminiGenerator{
		httpClient: server.Client(),
		apiKey:     "test-key",
		model:      "gemini-2.0-flash",
		baseURL:    server.URL,
	}

	resp, err := gen.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "test response" {
		t.Errorf("Text = %q, want %q", resp.Text, "test response")
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("TotalTokens = %d, want 8", resp.Usage.TotalTokens)
	}
}

func TestGeminiStream_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{"Hello ", "world", "!"}
		for _, chunk := range chunks {
			gemResp := geminiResponse{
				Candidates: []struct {
					Content      geminiContent `json:"content"`
					FinishReason string        `json:"finishReason"`
				}{
					{Content: geminiContent{Parts: []geminiPart{{Text: chunk}}}},
				},
			}
			data, _ := json.Marshal(gemResp)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
	}))
	defer server.Close()

	gen := &GeminiGenerator{
		httpClient: server.Client(),
		apiKey:     "test-key",
		model:      "gemini-2.0-flash",
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
	if collected != "Hello world!" {
		t.Errorf("collected = %q, want %q", collected, "Hello world!")
	}
}

func TestGeminiGenerate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	gen := &GeminiGenerator{
		httpClient: server.Client(),
		apiKey:     "test-key",
		model:      "gemini-2.0-flash",
		baseURL:    server.URL,
	}

	_, err := gen.Generate(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("error should mention status 404, got: %v", err)
	}
}

// --- Live integration tests ---

func TestGeminiOpener_Open_Integration(t *testing.T) {
	apiKey := os.Getenv(envGeminiAPIKey)
	if apiKey == "" {
		apiKey = os.Getenv(envGoogleAPIKey)
	}
	if apiKey == "" {
		t.Skip("skipping integration test: GEMINI_API_KEY or GOOGLE_API_KEY not set")
	}

	gen, err := Open(context.Background(), "gemini:///gemini-2.0-flash")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer gen.Close()

	resp, err := gen.Generate(context.Background(), "Respond with exactly: hello", WithMaxOutputTokens(10))
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text == "" {
		t.Error("Generate() returned empty text")
	}
	t.Logf("Response: %q", resp.Text)
}

func TestGeminiGenerator_Stream_Integration(t *testing.T) {
	apiKey := os.Getenv(envGeminiAPIKey)
	if apiKey == "" {
		apiKey = os.Getenv(envGoogleAPIKey)
	}
	if apiKey == "" {
		t.Skip("skipping integration test: GEMINI_API_KEY or GOOGLE_API_KEY not set")
	}

	gen, err := Open(context.Background(), "gemini:///gemini-2.0-flash")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer gen.Close()

	ch, err := gen.Stream(context.Background(), "Respond with exactly: hello", WithMaxOutputTokens(10))
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
	if collected == "" {
		t.Error("Stream() returned empty text")
	}
	t.Logf("Streamed response: %q", collected)
}
