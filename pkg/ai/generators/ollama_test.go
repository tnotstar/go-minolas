package generators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- Opener tests ---

func TestOllamaOpener_Id(t *testing.T) {
	op := &OllamaOpener{}
	if got := op.Id(); got != "ollama" {
		t.Errorf("OllamaOpener.Id() = %q, want %q", got, "ollama")
	}
}

func TestOllamaOpener_CanOpen(t *testing.T) {
	op := &OllamaOpener{}

	testCases := []struct {
		name   string
		rawurl string
		want   bool
	}{
		{name: "ollama scheme", rawurl: "ollama://", want: true},
		{name: "ollama with host", rawurl: "ollama://localhost:11434/", want: true},
		{name: "ollama with model", rawurl: "ollama:///llama3.2", want: true},
		{name: "gemini scheme", rawurl: "gemini://host", want: false},
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

func TestOllamaOpener_Open_NilURL(t *testing.T) {
	op := &OllamaOpener{}
	_, err := op.Open(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil URL, got nil")
	}
}

func TestOllamaOpener_Open_InvalidScheme(t *testing.T) {
	op := &OllamaOpener{}
	u, _ := url.Parse("gemini://host")
	_, err := op.Open(context.Background(), u)
	if err == nil {
		t.Fatal("expected error for invalid scheme, got nil")
	}
}

func TestOllamaOpener_Open_Defaults(t *testing.T) {
	op := &OllamaOpener{}
	u, _ := url.Parse("ollama://")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g, ok := gen.(*OllamaGenerator)
	if !ok {
		t.Fatal("expected *OllamaGenerator")
	}
	if g.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want %q", g.baseURL, "http://localhost:11434")
	}
	if g.model != defaultOllamaModel {
		t.Errorf("model = %q, want %q", g.model, defaultOllamaModel)
	}
}

func TestOllamaOpener_Open_HostAndModel(t *testing.T) {
	op := &OllamaOpener{}
	u, _ := url.Parse("ollama://myhost:8080/qwen3")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := gen.(*OllamaGenerator)
	if g.baseURL != "http://myhost:8080" {
		t.Errorf("baseURL = %q, want %q", g.baseURL, "http://myhost:8080")
	}
	if g.model != "qwen3" {
		t.Errorf("model = %q, want %q", g.model, "qwen3")
	}
}

func TestOllamaOpener_Open_WithUserinfo(t *testing.T) {
	op := &OllamaOpener{}
	u, _ := url.Parse("ollama://user:pass@myhost:8080/llama3.2")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := gen.(*OllamaGenerator)
	if g.baseURL != "http://user:pass@myhost:8080" {
		t.Errorf("baseURL = %q, want %q", g.baseURL, "http://user:pass@myhost:8080")
	}
}

// --- Request/response mapping tests ---

func TestOllamaBuildRequest(t *testing.T) {
	g := &OllamaGenerator{model: "llama3.2"}

	t.Run("minimal config", func(t *testing.T) {
		cfg := &Config{}
		req := g.buildRequest(cfg, "hello", false)
		if req.Prompt != "hello" {
			t.Errorf("Prompt = %q, want %q", req.Prompt, "hello")
		}
		if req.Stream != false {
			t.Error("expected stream=false")
		}
	})

	t.Run("full config sets all fields", func(t *testing.T) {
		cfg := &Config{
			Temperature:       0.8,
			MaxOutputTokens:   256,
			TopP:              0.9,
			TopK:              40,
			SystemInstruction: "You are a helpful assistant.",
			StopSequences:     []string{"END", "STOP"},
		}
		req := g.buildRequest(cfg, "test prompt", true)

		if req.Stream != true {
			t.Error("expected stream=true")
		}
		if *req.Options.Temperature != 0.8 {
			t.Errorf("Temperature = %v, want 0.8", *req.Options.Temperature)
		}
		if req.Options.NumPredict != 256 {
			t.Errorf("NumPredict = %d, want 256", req.Options.NumPredict)
		}
		if req.System != "You are a helpful assistant." {
			t.Errorf("System = %q, want %q", req.System, "You are a helpful assistant.")
		}
	})

	t.Run("config serializes to valid JSON", func(t *testing.T) {
		cfg := &Config{Temperature: 0.5, MaxOutputTokens: 100}
		req := g.buildRequest(cfg, "test", false)
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(data), `"temperature"`) {
			t.Error("JSON should contain temperature")
		}
		if !strings.Contains(string(data), `"num_predict"`) {
			t.Error("JSON should contain num_predict")
		}
	})
}

func TestOllamaMapResponse(t *testing.T) {
	g := &OllamaGenerator{}

	t.Run("full response maps correctly", func(t *testing.T) {
		ollResp := &ollamaResponse{
			Model: "llama3.2", Response: "Hello world", Done: true,
			DoneReason: "stop", PromptEvalCount: 10, EvalCount: 5,
		}
		resp := g.mapResponse(ollResp, "llama3.2")
		if resp.Text != "Hello world" {
			t.Errorf("Text = %q, want %q", resp.Text, "Hello world")
		}
		if resp.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
		}
	})

	t.Run("empty response", func(t *testing.T) {
		ollResp := &ollamaResponse{Done: true}
		resp := g.mapResponse(ollResp, "model")
		if resp.Text != "" {
			t.Errorf("Text = %q, want empty", resp.Text)
		}
	})
}

func TestOllamaResolveModel(t *testing.T) {
	g := &OllamaGenerator{model: "default-model"}

	if got := g.resolveModel(&Config{Model: "override"}); got != "override" {
		t.Errorf("with override = %q, want %q", got, "override")
	}
	if got := g.resolveModel(&Config{Model: ""}); got != "default-model" {
		t.Errorf("without override = %q, want %q", got, "default-model")
	}
}

// --- httptest.Server integration ---

func TestOllamaGenerate_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		resp := ollamaResponse{
			Model: req.Model, Response: "test response", Done: true,
			DoneReason: "stop", PromptEvalCount: 5, EvalCount: 3,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &OllamaGenerator{
		httpClient: server.Client(), baseURL: server.URL, model: "llama3.2",
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

func TestOllamaStream_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		chunks := []string{"Hello ", "world", "!"}
		for i, chunk := range chunks {
			resp := ollamaResponse{Model: "llama3.2", Response: chunk, Done: i == len(chunks)-1}
			if resp.Done {
				resp.DoneReason = "stop"
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "%s\n", data)
		}
	}))
	defer server.Close()

	gen := &OllamaGenerator{
		httpClient: server.Client(), baseURL: server.URL, model: "llama3.2",
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

func TestOllamaGenerate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "model not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	gen := &OllamaGenerator{
		httpClient: server.Client(), baseURL: server.URL, model: "nonexistent",
	}

	_, err := gen.Generate(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("error should mention status 404, got: %v", err)
	}
}

// --- Live integration test ---

func TestOllamaGenerate_Integration(t *testing.T) {
	gen, err := Open(context.Background(), "ollama://localhost:11434/"+defaultOllamaModel)
	if err != nil {
		t.Skipf("skipping integration test: Ollama not available: %v", err)
	}
	defer gen.Close()

	resp, err := gen.Generate(context.Background(), "Respond with exactly: hello", WithMaxOutputTokens(10))
	if err != nil {
		t.Skipf("skipping integration test: Ollama request failed: %v", err)
	}
	if resp.Text == "" {
		t.Error("Generate() returned empty text")
	}
	t.Logf("Response: %q", resp.Text)
}

func TestOllamaStream_Integration(t *testing.T) {
	gen, err := Open(context.Background(), "ollama://localhost:11434/"+defaultOllamaModel)
	if err != nil {
		t.Skipf("skipping integration test: Ollama not available: %v", err)
	}
	defer gen.Close()

	ch, err := gen.Stream(context.Background(), "Respond with exactly: hello", WithMaxOutputTokens(10))
	if err != nil {
		t.Skipf("skipping integration test: Ollama stream failed: %v", err)
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
