package generators

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

// --- Opener tests ---

func TestCompatOpener_Id(t *testing.T) {
	op := &CompatOpener{}
	if got := op.Id(); got != "compat" {
		t.Errorf("CompatOpener.Id() = %q, want %q", got, "compat")
	}
}

func TestCompatOpener_CanOpen(t *testing.T) {
	op := &CompatOpener{}

	testCases := []struct {
		name   string
		rawurl string
		want   bool
	}{
		{name: "compat scheme", rawurl: "compat://localhost/m", want: true},
		{name: "openai scheme", rawurl: "openai://", want: false},
		{name: "https scheme", rawurl: "https://example.com", want: false},
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

func TestCompatOpener_Open_NilURL(t *testing.T) {
	op := &CompatOpener{}
	if _, err := op.Open(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil URL, got nil")
	}
}

func TestCompatOpener_Open_RequiresHost(t *testing.T) {
	op := &CompatOpener{}
	u, _ := url.Parse("compat:///mymodel") // no host
	if _, err := op.Open(context.Background(), u); err == nil {
		t.Fatal("expected error for missing host, got nil")
	}
}

func TestCompatOpener_Open_RequiresModel(t *testing.T) {
	op := &CompatOpener{}
	u, _ := url.Parse("compat://localhost") // no model
	if _, err := op.Open(context.Background(), u); err == nil {
		t.Fatal("expected error for missing model, got nil")
	}
}

func TestCompatOpener_Open_ExplicitHostAndModel(t *testing.T) {
	op := &CompatOpener{}
	u, _ := url.Parse("compat://localhost:1234/v1/custom-model")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g, ok := gen.(*CompatGenerator)
	if !ok {
		t.Fatal("expected *CompatGenerator")
	}
	// compat always uses https for the wire transport (ADR 4: no unencrypted
	// HTTP for service-to-service), regardless of the host being local.
	if g.baseURL != "https://localhost:1234/v1" {
		t.Errorf("baseURL = %q, want https://localhost:1234/v1", g.baseURL)
	}
	if g.model != "custom-model" {
		t.Errorf("model = %q, want custom-model", g.model)
	}
}

func TestCompatOpener_Open_HttpsHost(t *testing.T) {
	op := &CompatOpener{}
	// compat uses https scheme by default for the wire transport.
	u, _ := url.Parse("compat://api.openrouter.ai/api/v1/openai/gpt-4o")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := gen.(*CompatGenerator)
	if g.baseURL != "https://api.openrouter.ai/api/v1/openai" {
		t.Errorf("baseURL = %q, want https://api.openrouter.ai/api/v1/openai", g.baseURL)
	}
	if g.model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", g.model)
	}
}

// --- API key lookup tests ---

func TestLookupCompatAPIKey(t *testing.T) {
	origCompat := os.Getenv(envCompatAPIKey)
	origOpenAI := os.Getenv(envOpenAIAPIKey)
	t.Cleanup(func() {
		os.Setenv(envCompatAPIKey, origCompat)
		os.Setenv(envOpenAIAPIKey, origOpenAI)
	})

	testCases := []struct {
		name    string
		envSet  map[string]string
		wantKey string
	}{
		{name: "COMPAT_API_KEY wins", envSet: map[string]string{envCompatAPIKey: "compat-k", envOpenAIAPIKey: "openai-k"}, wantKey: "compat-k"},
		{name: "falls back to OPENAI_API_KEY", envSet: map[string]string{envOpenAIAPIKey: "openai-k"}, wantKey: "openai-k"},
		{name: "no key", envSet: map[string]string{}, wantKey: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(envCompatAPIKey)
			os.Unsetenv(envOpenAIAPIKey)
			for k, v := range tc.envSet {
				os.Setenv(k, v)
			}
			if got := lookupCompatAPIKey(); got != tc.wantKey {
				t.Errorf("lookupCompatAPIKey() = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

// --- httptest.Server integration ---

func TestCompatGenerate_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer compat-k" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	gen := &CompatGenerator{
		httpClient: server.Client(),
		apiKey:     "compat-k",
		model:      "custom-model",
		baseURL:    server.URL,
	}

	resp, err := gen.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "hi" {
		t.Errorf("Text = %q, want hi", resp.Text)
	}
	if resp.Usage.TotalTokens != 2 {
		t.Errorf("TotalTokens = %d, want 2", resp.Usage.TotalTokens)
	}
}

func TestCompatGenerate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"nope"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	gen := &CompatGenerator{
		httpClient: server.Client(),
		apiKey:     "compat-k",
		model:      "m",
		baseURL:    server.URL,
	}

	if _, err := gen.Generate(context.Background(), "hello"); err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestCompatStream_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{"A", "B"} {
			payload := fmt.Sprintf(`{"choices":[{"delta":{"content":%q}}]}`, chunk)
			fmt.Fprintf(w, "data: %s\n\n", payload)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	gen := &CompatGenerator{
		httpClient: server.Client(),
		apiKey:     "compat-k",
		model:      "m",
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
	if collected != "AB" {
		t.Errorf("collected = %q, want AB", collected)
	}
}

func TestCompatGenerator_Close(t *testing.T) {
	gen := &CompatGenerator{}
	if err := gen.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
