package generators

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// --- Internal JSON types for the OpenAI Chat Completions wire format ---
//
// These types are shared by the openai, compat, and azure_openai providers,
// which all speak the same request/response shape. Each provider keeps its own
// Opener and Generator types (distinct URL-scheme dispatch and authentication)
// but delegates wire-format work to the helpers below.

// chatCompletionMessage is a single message in the Chat Completions format.
type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionRequest is the request body posted to /v1/chat/completions.
type chatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Stream      bool                    `json:"stream,omitempty"`
	Temperature *float32                `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
	TopP        *float32                `json:"top_p,omitempty"`
	Stop        []string                `json:"stop,omitempty"`
}

// chatCompletionResponse is the non-streaming response from Chat Completions.
type chatCompletionResponse struct {
	Choices []struct {
		Message      chatCompletionMessage `json:"message"`
		Delta        chatCompletionMessage `json:"delta"`
		FinishReason string                `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// buildChatCompletionRequest converts a Config and prompt into a Chat Completions
// request. The stream flag is set by the caller after construction.
func buildChatCompletionRequest(cfg *Config, prompt string) chatCompletionRequest {
	messages := make([]chatCompletionMessage, 0, 2)
	if cfg.SystemInstruction != "" {
		messages = append(messages, chatCompletionMessage{Role: "system", Content: cfg.SystemInstruction})
	}
	messages = append(messages, chatCompletionMessage{Role: "user", Content: prompt})

	req := chatCompletionRequest{
		Model:    cfg.Model,
		Messages: messages,
	}

	if cfg.Temperature != 0 {
		req.Temperature = ptrFloat32(cfg.Temperature)
	}
	if cfg.MaxOutputTokens != 0 {
		req.MaxTokens = cfg.MaxOutputTokens
	}
	if cfg.TopP != 0 {
		req.TopP = ptrFloat32(cfg.TopP)
	}
	// Note: OpenAI Chat Completions has no top_k; Config.TopK is intentionally
	// ignored here (see design.md D7).
	if len(cfg.StopSequences) > 0 {
		req.Stop = cfg.StopSequences
	}

	return req
}

// parseChatCompletionResponse decodes a Chat Completions response body and maps
// it into a generators.Response. model is the model name used for the request
// (used to populate Response.Model when the body omits it).
func parseChatCompletionResponse(body []byte, model string) (*Response, error) {
	var ccResp chatCompletionResponse
	if err := json.Unmarshal(body, &ccResp); err != nil {
		return nil, fmt.Errorf("generators: chat completion decode response: %w", err)
	}

	out := &Response{Model: model}
	if len(ccResp.Choices) > 0 {
		out.Text = ccResp.Choices[0].Message.Content
		out.FinishReason = ccResp.Choices[0].FinishReason
	}
	if ccResp.Usage != nil {
		out.Usage = Usage{
			PromptTokens:     ccResp.Usage.PromptTokens,
			CompletionTokens: ccResp.Usage.CompletionTokens,
			TotalTokens:      ccResp.Usage.TotalTokens,
		}
	}
	return out, nil
}

// consumeChatCompletionSSE reads Server-Sent Events from the response body and
// sends parsed chunks on the channel. Each data line carries a delta payload;
// the stream terminates at the `data: [DONE]` sentinel.
func consumeChatCompletionSSE(ctx context.Context, body io.Reader, ch chan<- StreamChunk) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			ch <- StreamChunk{Error: ctx.Err()}
			return
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		var ccResp chatCompletionResponse
		if err := json.Unmarshal([]byte(data), &ccResp); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("generators: chat completion SSE unmarshal: %w", err)}
			return
		}

		if len(ccResp.Choices) > 0 && ccResp.Choices[0].Delta.Content != "" {
			ch <- StreamChunk{Text: ccResp.Choices[0].Delta.Content}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("generators: chat completion SSE read: %w", err)}
	}
}
