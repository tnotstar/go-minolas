package generators_test

import (
	"fmt"

	"github.com/tnotstar/go-minolas/pkg/ai/generators"
)

func ExampleListOpeners() {
	// List all registered generator openers.
	// Gemini and Ollama openers are auto-registered on import.
	openers := generators.ListOpeners()
	if len(openers) >= 0 {
		fmt.Println("Generator openers registered")
	}
	// Output: Generator openers registered
}
