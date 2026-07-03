package generators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

// --- Opener tests ---

func TestOpenAIOpener_Id(t *testing.T) {
	op := &OpenAIOpener{}
	if got := op.Id(); got != "openai" {
		t.Errorf("OpenAIOpener.Id() = %q, want %q", got, "openai")
	}
}

func TestOpenAIOpener_CanOpen(t *testing.T) {
	op := &OpenAIOpener{}

	testCases := []struct {
		name   string
		rawurl string
		want   bool
	}{
		{name: "openai scheme", rawurl: "openai://", want: true},
		{name: "openai with model", rawurl: "openai:///gpt-4o", want: true},
		{name: "compat scheme", rawurl: "compat://localhost/m", want: false},
		{name: "anthropic scheme", rawurl: "anthropic://", want: false},
		{name: "https scheme", rawurl: "https://api.openai.com", want: false},
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

func TestOpenAIOpener_Open_NilURL(t *testing.T) {
	op := &OpenAIOpener{}
	if _, err := op.Open(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil URL, got nil")
	}
}

func TestOpenAIOpener_Open_InvalidScheme(t *testing.T) {
	op := &OpenAIOpener{}
	u, _ := url.Parse("ollama://localhost")
	if _, err := op.Open(context.Background(), u); err == nil {
		t.Fatal("expected error for invalid scheme, got nil")
	}
}

func TestOpenAIOpener_Open_Defaults(t *testing.T) {
	op := &OpenAIOpener{}
	u, _ := url.Parse("openai://")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g, ok := gen.(*OpenAIGenerator)
	if !ok {
		t.Fatal("expected *OpenAIGenerator")
	}
	if g.baseURL != "https://api.openai.com" {
		t.Errorf("baseURL = %q, want https://api.openai.com", g.baseURL)
	}
	if g.model != defaultOpenAIModel {
		t.Errorf("model = %q, want %q", g.model, defaultOpenAIModel)
	}
}

func TestOpenAIOpener_Open_ModelOnly(t *testing.T) {
	op := &OpenAIOpener{}
	u, _ := url.Parse("openai:///gpt-4o")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g := gen.(*OpenAIGenerator)
	if g.model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", g.model)
	}
}

// --- API key lookup tests ---

func TestLookupOpenAIAPIKey(t *testing.T) {
	orig := os.Getenv(envOpenAIAPIKey)
	t.Cleanup(func() { os.Setenv(envOpenAIAPIKey, orig) })

	testCases := []struct {
		name    string
		envSet  map[string]string
		wantKey string
	}{
		{name: "OPENAI_API_KEY set", envSet: map[string]string{envOpenAIAPIKey: "sk-test"}, wantKey: "sk-test"},
		{name: "no key", envSet: map[string]string{}, wantKey: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(envOpenAIAPIKey)
			for k, v := range tc.envSet {
				os.Setenv(k, v)
			}
			if got := lookupOpenAIAPIKey(); got != tc.wantKey {
				t.Errorf("lookupOpenAIAPIKey() = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

// --- httptest.Server integration ---

func TestOpenAIGenerate_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req chatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) == 0 || req.Messages[len(req.Messages)-1].Content != "hello" {
			http.Error(w, "bad prompt", http.StatusBadRequest)
			return
		}
		resp := chatCompletionResponse{
			Choices: []struct {
				Message      chatCompletionMessage `json:"message"`
				Delta        chatCompletionMessage `json:"delta"`
				FinishReason string                `json:"finish_reason"`
			}{
				{
					Message:      chatCompletionMessage{Role: "assistant", Content: "test response"},
					FinishReason: "stop",
				},
			},
			Usage: &struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &OpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-test",
		model:      "gpt-4o-mini",
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
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", resp.FinishReason)
	}
}

func TestOpenAIGenerate_SystemInstructionSentAsSystemMessage(t *testing.T) {
	var seenRoles []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		for _, m := range req.Messages {
			seenRoles = append(seenRoles, m.Role)
		}
		resp := chatCompletionResponse{}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &OpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-test",
		model:      "gpt-4o-mini",
		baseURL:    server.URL,
	}

	if _, err := gen.Generate(context.Background(), "hi", WithSystemInstruction("be brief")); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(seenRoles) != 2 || seenRoles[0] != "system" || seenRoles[1] != "user" {
		t.Errorf("roles = %v, want [system user]", seenRoles)
	}
}

func TestOpenAIGenerate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	gen := &OpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-test",
		model:      "gpt-4o-mini",
		baseURL:    server.URL,
	}

	resp, err := gen.Generate(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on error, got %+v", resp)
	}
	if !contains(err.Error(), "429") {
		t.Errorf("error %q should mention status 429", err)
	}
}

func TestOpenAIStream_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{"Hello ", "world", "!"}
		for _, chunk := range chunks {
			payload := fmt.Sprintf(`{"choices":[{"delta":{"content":%q}}]}`, chunk)
			fmt.Fprintf(w, "data: %s\n\n", payload)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	gen := &OpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "sk-test",
		model:      "gpt-4o-mini",
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

func TestOpenAIGenerator_Close(t *testing.T) {
	gen := &OpenAIGenerator{}
	if err := gen.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// contains is a minimal substring helper used by error-mapping assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
