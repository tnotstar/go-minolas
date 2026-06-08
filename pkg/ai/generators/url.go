package generators

import (
	"net/url"
	"strings"
)

// providerConfig holds the parsed result of a provider URL.
type providerConfig struct {
	baseURL string
	model   string
}

// parseProviderURL extracts the base URL and model from a provider URL.
//
// The URL format is: {scheme}://[userinfo@]{host}[:port]/{path_prefix...}/{model}
//
// Rules:
//   - The scheme identifies the opener (e.g., "gemini", "ollama").
//   - The host (netloc) defaults to defaultHost if empty.
//   - The last path segment is the model; defaults to defaultModel if empty.
//   - The remaining path prefix defaults to defaultPathPrefix if empty.
//   - The constructed base URL uses the given HTTP scheme, with optional userinfo.
//   - Username/password from userinfo are passed as part of the base URL.
func parseProviderURL(u *url.URL, httpScheme, defaultHost, defaultPathPrefix, defaultModel string) providerConfig {
	host := u.Host
	if host == "" {
		host = defaultHost
	}

	pathPrefix, model := splitLastSegment(u.EscapedPath())
	if pathPrefix == "" {
		pathPrefix = defaultPathPrefix
	}
	if model == "" {
		model = defaultModel
	}

	basePath := ""
	if pathPrefix != "" {
		basePath = "/" + pathPrefix
	}

	base := &url.URL{
		Scheme: httpScheme,
		Host:   host,
		Path:   basePath,
	}
	if u.User != nil {
		base.User = u.User
	}

	return providerConfig{
		baseURL: base.String(),
		model:   model,
	}
}

// splitLastSegment splits a URL path into prefix and last segment.
//
// Examples:
//   - "/v1beta/models/gemini-2.0-flash" → ("v1beta/models", "gemini-2.0-flash")
//   - "/llama3.2"                       → ("", "llama3.2")
//   - "/"                               → ("", "")
//   - ""                                → ("", "")
func splitLastSegment(path string) (prefix, last string) {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", ""
	}
	idx := strings.LastIndex(trimmed, "/")
	if idx == -1 {
		return "", trimmed
	}
	return trimmed[:idx], trimmed[idx+1:]
}
