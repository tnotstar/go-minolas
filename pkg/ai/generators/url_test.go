package generators

import (
	"net/url"
	"testing"
)

func TestSplitLastSegment(t *testing.T) {
	testCases := []struct {
		name      string
		path      string
		wantPre   string
		wantLast  string
	}{
		{name: "empty path", path: "", wantPre: "", wantLast: ""},
		{name: "slash only", path: "/", wantPre: "", wantLast: ""},
		{name: "single segment", path: "/llama3.2", wantPre: "", wantLast: "llama3.2"},
		{name: "two segments", path: "/models/gemini-2.0-flash", wantPre: "models", wantLast: "gemini-2.0-flash"},
		{name: "three segments", path: "/v1beta/models/gemini-2.0-flash", wantPre: "v1beta/models", wantLast: "gemini-2.0-flash"},
		{name: "trailing slash", path: "/v1beta/models/", wantPre: "v1beta/models", wantLast: ""},
		{name: "segment without leading slash", path: "single", wantPre: "", wantLast: "single"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotPre, gotLast := splitLastSegment(tc.path)
			if gotPre != tc.wantPre {
				t.Errorf("prefix = %q, want %q", gotPre, tc.wantPre)
			}
			if gotLast != tc.wantLast {
				t.Errorf("last = %q, want %q", gotLast, tc.wantLast)
			}
		})
	}
}

func TestParseProviderURL(t *testing.T) {
	testCases := []struct {
		name             string
		rawurl           string
		httpScheme       string
		defaultHost      string
		defaultPathPre   string
		defaultModel     string
		wantBaseURL      string
		wantModel        string
	}{
		{
			name:           "gemini minimal",
			rawurl:         "gemini://",
			httpScheme:     "https",
			defaultHost:    "generativelanguage.googleapis.com",
			defaultPathPre: "v1beta/models",
			defaultModel:   "gemini-2.0-flash",
			wantBaseURL:    "https://generativelanguage.googleapis.com/v1beta/models",
			wantModel:      "gemini-2.0-flash",
		},
		{
			name:           "gemini with model only",
			rawurl:         "gemini:///gemini-2.0-pro",
			httpScheme:     "https",
			defaultHost:    "generativelanguage.googleapis.com",
			defaultPathPre: "v1beta/models",
			defaultModel:   "gemini-2.0-flash",
			wantBaseURL:    "https://generativelanguage.googleapis.com/v1beta/models",
			wantModel:      "gemini-2.0-pro",
		},
		{
			name:           "gemini fully specified",
			rawurl:         "gemini://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash",
			httpScheme:     "https",
			defaultHost:    "generativelanguage.googleapis.com",
			defaultPathPre: "v1beta/models",
			defaultModel:   "gemini-2.0-flash",
			wantBaseURL:    "https://generativelanguage.googleapis.com/v1beta/models",
			wantModel:      "gemini-2.0-flash",
		},
		{
			name:           "gemini custom host with model",
			rawurl:         "gemini://custom.api.com/v1/models/gpt-4",
			httpScheme:     "https",
			defaultHost:    "generativelanguage.googleapis.com",
			defaultPathPre: "v1beta/models",
			defaultModel:   "gemini-2.0-flash",
			wantBaseURL:    "https://custom.api.com/v1/models",
			wantModel:      "gpt-4",
		},
		{
			name:           "ollama minimal",
			rawurl:         "ollama://",
			httpScheme:     "http",
			defaultHost:    "localhost:11434",
			defaultPathPre: "",
			defaultModel:   "llama3.2",
			wantBaseURL:    "http://localhost:11434",
			wantModel:      "llama3.2",
		},
		{
			name:           "ollama with host and model",
			rawurl:         "ollama://localhost:11434/llama3.2",
			httpScheme:     "http",
			defaultHost:    "localhost:11434",
			defaultPathPre: "",
			defaultModel:   "llama3.2",
			wantBaseURL:    "http://localhost:11434",
			wantModel:      "llama3.2",
		},
		{
			name:           "ollama remote with auth",
			rawurl:         "ollama://user:pass@my-server:8080/qwen3",
			httpScheme:     "http",
			defaultHost:    "localhost:11434",
			defaultPathPre: "",
			defaultModel:   "llama3.2",
			wantBaseURL:    "http://user:pass@my-server:8080",
			wantModel:      "qwen3",
		},
		{
			name:           "ollama host only uses default model",
			rawurl:         "ollama://localhost:11434",
			httpScheme:     "http",
			defaultHost:    "localhost:11434",
			defaultPathPre: "",
			defaultModel:   "llama3.2",
			wantBaseURL:    "http://localhost:11434",
			wantModel:      "llama3.2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawurl)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", tc.rawurl, err)
			}

			cfg := parseProviderURL(u, tc.httpScheme, tc.defaultHost, tc.defaultPathPre, tc.defaultModel)

			if cfg.baseURL != tc.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", cfg.baseURL, tc.wantBaseURL)
			}
			if cfg.model != tc.wantModel {
				t.Errorf("model = %q, want %q", cfg.model, tc.wantModel)
			}
		})
	}
}
