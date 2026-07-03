package generators

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// --- Opener tests ---

func TestAzureOpenAIOpener_Id(t *testing.T) {
	op := &AzureOpenAIOpener{}
	if got := op.Id(); got != "azure-openai" {
		t.Errorf("AzureOpenAIOpener.Id() = %q, want %q", got, "azure-openai")
	}
}

func TestAzureOpenAIOpener_CanOpen(t *testing.T) {
	op := &AzureOpenAIOpener{}

	testCases := []struct {
		name   string
		rawurl string
		want   bool
	}{
		{name: "azure-openai scheme", rawurl: "azure-openai://res.cognitiveservices.azure.com/dep", want: true},
		{name: "openai scheme", rawurl: "openai://", want: false},
		{name: "compat scheme", rawurl: "compat://h/m", want: false},
		{name: "https scheme", rawurl: "https://res.cognitiveservices.azure.com", want: false},
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

func TestAzureOpenAIOpener_Open_NilURL(t *testing.T) {
	op := &AzureOpenAIOpener{}
	if _, err := op.Open(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil URL, got nil")
	}
}

func TestAzureOpenAIOpener_Open_RequiresHost(t *testing.T) {
	op := &AzureOpenAIOpener{}
	u, _ := url.Parse("azure-openai:///my-deploy") // no host
	if _, err := op.Open(context.Background(), u); err == nil {
		t.Fatal("expected error for missing host, got nil")
	}
}

func TestAzureOpenAIOpener_Open_RequiresDeployment(t *testing.T) {
	op := &AzureOpenAIOpener{}
	u, _ := url.Parse("azure-openai://res.cognitiveservices.azure.com") // no deployment
	if _, err := op.Open(context.Background(), u); err == nil {
		t.Fatal("expected error for missing deployment, got nil")
	}
}

func TestAzureOpenAIOpener_Open_ParsesResourceAndDeployment(t *testing.T) {
	op := &AzureOpenAIOpener{}
	u, _ := url.Parse("azure-openai://my-resource.cognitiveservices.azure.com/my-deployment")

	gen, err := op.Open(context.Background(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g, ok := gen.(*AzureOpenAIGenerator)
	if !ok {
		t.Fatal("expected *AzureOpenAIGenerator")
	}
	if g.baseURL != "https://my-resource.cognitiveservices.azure.com" {
		t.Errorf("baseURL = %q, want https://my-resource.cognitiveservices.azure.com", g.baseURL)
	}
	if g.deployment != "my-deployment" {
		t.Errorf("deployment = %q, want my-deployment", g.deployment)
	}
	if g.apiVersion != defaultAzureOpenAIAPIVersion {
		t.Errorf("apiVersion = %q, want %q", g.apiVersion, defaultAzureOpenAIAPIVersion)
	}
}

// --- API key lookup tests ---

func TestLookupAzureOpenAIAPIKey(t *testing.T) {
	orig := os.Getenv(envAzureOpenAIAPIKey)
	t.Cleanup(func() { os.Setenv(envAzureOpenAIAPIKey, orig) })

	testCases := []struct {
		name    string
		envSet  map[string]string
		wantKey string
	}{
		{name: "AZURE_OPENAI_API_KEY set", envSet: map[string]string{envAzureOpenAIAPIKey: "az-key"}, wantKey: "az-key"},
		{name: "no key", envSet: map[string]string{}, wantKey: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(envAzureOpenAIAPIKey)
			for k, v := range tc.envSet {
				os.Setenv(k, v)
			}
			if got := lookupAzureOpenAIAPIKey(); got != tc.wantKey {
				t.Errorf("lookupAzureOpenAIAPIKey() = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

// --- endpoint construction ---

func TestAzureOpenAIGenerator_EndpointConstruction(t *testing.T) {
	gen := &AzureOpenAIGenerator{
		baseURL:    "https://my-resource.cognitiveservices.azure.com",
		deployment: "my-deployment",
		apiVersion: "2024-10-21",
	}
	endpoint := gen.endpoint()
	want := "https://my-resource.cognitiveservices.azure.com/openai/deployments/my-deployment/chat/completions?api-version=2024-10-21"
	if endpoint != want {
		t.Errorf("endpoint = %q, want %q", endpoint, want)
	}
}

// --- httptest.Server integration ---

func TestAzureOpenAIGenerate_HTTPTestServer(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		if r.Header.Get("api-key") != "az-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			http.Error(w, "must not send Authorization", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	gen := &AzureOpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "az-key",
		baseURL:    server.URL,
		deployment: "dep",
		apiVersion: defaultAzureOpenAIAPIVersion,
	}

	resp, err := gen.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "hi" {
		t.Errorf("Text = %q, want hi", resp.Text)
	}
	if !strings.Contains(gotPath, "/openai/deployments/dep/chat/completions") {
		t.Errorf("path = %q, want deployment-scoped chat completions path", gotPath)
	}
	if !strings.Contains(gotPath, "api-version="+defaultAzureOpenAIAPIVersion) {
		t.Errorf("path = %q, want api-version query param", gotPath)
	}
}

func TestAzureOpenAIGenerate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"nope"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	gen := &AzureOpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "az-key",
		baseURL:    server.URL,
		deployment: "dep",
		apiVersion: defaultAzureOpenAIAPIVersion,
	}

	if _, err := gen.Generate(context.Background(), "hello"); err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestAzureOpenAIStream_HTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{"X", "Y"} {
			payload := fmt.Sprintf(`{"choices":[{"delta":{"content":%q}}]}`, chunk)
			fmt.Fprintf(w, "data: %s\n\n", payload)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	gen := &AzureOpenAIGenerator{
		httpClient: server.Client(),
		apiKey:     "az-key",
		baseURL:    server.URL,
		deployment: "dep",
		apiVersion: defaultAzureOpenAIAPIVersion,
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
	if collected != "XY" {
		t.Errorf("collected = %q, want XY", collected)
	}
}

func TestAzureOpenAIGenerator_Close(t *testing.T) {
	gen := &AzureOpenAIGenerator{}
	if err := gen.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
