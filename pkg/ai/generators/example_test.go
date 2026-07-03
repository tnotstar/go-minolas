package generators_test

import (
	"testing"

	"github.com/tnotstar/go-minolas/pkg/ai/generators"
)

// TestPublicListOpeners documents the public surface of generators.ListOpeners.
// The built-in providers (gemini, ollama, openai, compat, azure-openai,
// anthropic) self-register on package init; the exact count observed in a test
// depends on order because internal tests call ResetOpeners, so we only assert
// that the call is safe and returns a slice.
func TestPublicListOpeners(t *testing.T) {
	openers := generators.ListOpeners()
	if openers == nil {
		t.Error("ListOpeners() returned nil slice")
	}
}
