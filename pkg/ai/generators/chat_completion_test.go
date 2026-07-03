package generators

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBuildChatCompletionRequest(t *testing.T) {
	t.Run("minimal config produces only user message", func(t *testing.T) {
		cfg := &Config{}
		req := buildChatCompletionRequest(cfg, "hello")
		if len(req.Messages) != 1 {
			t.Fatalf("Messages len = %d, want 1", len(req.Messages))
		}
		if req.Messages[0].Role != "user" || req.Messages[0].Content != "hello" {
			t.Errorf("first message = %+v, want role=user content=hello", req.Messages[0])
		}
		if req.Temperature != nil || req.MaxTokens != 0 || req.TopP != nil {
			t.Error("expected no options for minimal config")
		}
	})

	t.Run("system instruction prepends system message", func(t *testing.T) {
		cfg := &Config{SystemInstruction: "be brief"}
		req := buildChatCompletionRequest(cfg, "hi")
		if len(req.Messages) != 2 {
			t.Fatalf("Messages len = %d, want 2", len(req.Messages))
		}
		if req.Messages[0].Role != "system" || req.Messages[0].Content != "be brief" {
			t.Errorf("system message = %+v", req.Messages[0])
		}
		if req.Messages[1].Role != "user" || req.Messages[1].Content != "hi" {
			t.Errorf("user message = %+v", req.Messages[1])
		}
	})

	t.Run("options map to wire fields", func(t *testing.T) {
		cfg := &Config{
			Model:           "gpt-4o",
			Temperature:     0.7,
			MaxOutputTokens: 256,
			TopP:            0.9,
			StopSequences:   []string{"END"},
		}
		req := buildChatCompletionRequest(cfg, "prompt")
		if req.Model != "gpt-4o" {
			t.Errorf("Model = %q, want gpt-4o", req.Model)
		}
		if req.Temperature == nil || *req.Temperature != 0.7 {
			t.Errorf("Temperature = %v, want 0.7", req.Temperature)
		}
		if req.MaxTokens != 256 {
			t.Errorf("MaxTokens = %d, want 256", req.MaxTokens)
		}
		if req.TopP == nil || *req.TopP != 0.9 {
			t.Errorf("TopP = %v, want 0.9", req.TopP)
		}
		if len(req.Stop) != 1 || req.Stop[0] != "END" {
			t.Errorf("Stop = %v, want [END]", req.Stop)
		}
	})

	t.Run("top_k is ignored", func(t *testing.T) {
		cfg := &Config{TopK: 40}
		req := buildChatCompletionRequest(cfg, "prompt")
		// TopK must not be reflected in the request; the Chat Completions API
		// has no top_k field. Serializing and checking is the safest assertion.
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(data), "top_k") {
			t.Errorf("request must not contain top_k, got %s", data)
		}
	})
}

func TestParseChatCompletionResponse(t *testing.T) {
	t.Run("maps text finish reason and usage", func(t *testing.T) {
		body := []byte(`{
			"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}
		}`)
		resp, err := parseChatCompletionResponse(body, "gpt-4o")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "hi" {
			t.Errorf("Text = %q, want hi", resp.Text)
		}
		if resp.Model != "gpt-4o" {
			t.Errorf("Model = %q, want gpt-4o", resp.Model)
		}
		if resp.FinishReason != "stop" {
			t.Errorf("FinishReason = %q, want stop", resp.FinishReason)
		}
		if resp.Usage.TotalTokens != 3 {
			t.Errorf("TotalTokens = %d, want 3", resp.Usage.TotalTokens)
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		_, err := parseChatCompletionResponse([]byte("{not json"), "m")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestConsumeChatCompletionSSE(t *testing.T) {
	t.Run("emits delta chunks until DONE", func(t *testing.T) {
		deltas := []string{"Hello ", "world"}
		var sb strings.Builder
		for _, d := range deltas {
			payload := fmt.Sprintf(`{"choices":[{"delta":{"content":%q}}]}`, d)
			fmt.Fprintf(&sb, "data: %s\n\n", payload)
		}
		sb.WriteString("data: [DONE]\n\n")

		ch := make(chan StreamChunk, 4)
		consumeChatCompletionSSE(context.Background(), strings.NewReader(sb.String()), ch)
		close(ch)

		var got string
		for c := range ch {
			if c.Error != nil {
				t.Fatalf("unexpected error: %v", c.Error)
			}
			got += c.Text
		}
		if got != "Hello world" {
			t.Errorf("collected = %q, want %q", got, "Hello world")
		}
	})

	t.Run("skips non-data lines", func(t *testing.T) {
		body := ":ping\n\nevent: hello\ndata: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\ndata: [DONE]\n\n"
		ch := make(chan StreamChunk, 2)
		consumeChatCompletionSSE(context.Background(), strings.NewReader(body), ch)
		close(ch)

		var got string
		for c := range ch {
			got += c.Text
		}
		if got != "x" {
			t.Errorf("collected = %q, want x", got)
		}
	})

	t.Run("propagates malformed data line as error", func(t *testing.T) {
		ch := make(chan StreamChunk, 2)
		consumeChatCompletionSSE(context.Background(), strings.NewReader("data: not-json\n\n"), ch)
		close(ch)
		var sawErr bool
		for c := range ch {
			if c.Error != nil {
				sawErr = true
			}
		}
		if !sawErr {
			t.Error("expected an error chunk for malformed data")
		}
	})
}
