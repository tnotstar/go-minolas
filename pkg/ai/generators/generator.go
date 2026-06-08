package generators

import "context"

// Generator defines the interface for AI text generation providers.
// Each provider opener creates a Generator via its Open method.
type Generator interface {

	// Generate produces a text completion for the given prompt.
	Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error)

	// Stream produces a streaming text completion for the given prompt.
	// Returns a read-only channel that yields response chunks as they arrive.
	// The channel is closed when the stream ends; the final chunk may contain
	// a non-nil Error to signal stream failure.
	Stream(ctx context.Context, prompt string, opts ...Option) (<-chan StreamChunk, error)

	// Close releases any resources held by the generator.
	Close() error
}

// Response represents the result of a text generation call.
type Response struct {
	Text         string
	Model        string
	FinishReason string
	Usage        Usage
}

// Usage represents token usage statistics for a generation call.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// StreamChunk represents a single chunk of a streamed response.
type StreamChunk struct {
	Text  string
	Error error
}
