# Tasks - add-openai-anthropic-generators

> TDD discipline: for each driver, write the failing test first, then implement until green.
> Run `go test ./pkg/ai/generators/` after each driver.

## 1. Shared Chat Completions helper

- [ ] 1.1 Create `pkg/ai/generators/chat_completion.go` with unexported types `chatCompletionRequest`, `chatCompletionMessage`, `chatCompletionResponse`, `chatCompletionDelta`, and helpers `buildChatCompletionRequest(cfg *Config, prompt string) chatCompletionRequest`, `parseChatCompletionResponse(body []byte, model string) (*Response, error)`, `consumeChatCompletionSSE(ctx context.Context, body io.Reader, ch chan<- StreamChunk)`
- [ ] 1.2 Add `chat_completion_test.go` covering request building (system message, option mapping), response parsing (usage, finish reason), and SSE consumption (delta chunks, `[DONE]` terminator)

## 2. OpenAI driver (`openai://`)

- [ ] 2.1 Write `openai_test.go` (Red): URL parsing (defaults, explicit host/model), `Generate` happy path via `httptest.Server`, HTTP error mapping, `Usage` mapping, `Stream` over SSE, `Close`
- [ ] 2.2 Implement `openai.go` until tests pass (Green): `OpenAIOpener` (`Id`/`CanOpen`/`Open`), `OpenAIGenerator`, `OPENAI_API_KEY` env auth, `Authorization: Bearer` header, delegate to shared helper, `init()` registration

## 3. Compat driver (`compat://`)

- [ ] 3.1 Write `compat_test.go` (Red): URL parsing (host+model required → error if missing), `Generate`, streaming, `COMPAT_API_KEY` → `OPENAI_API_KEY` fallback resolution
- [ ] 3.2 Implement `compat.go` until tests pass (Green): `CompatOpener`, `CompatGenerator`, required-host/model validation, env fallback, delegate to shared helper

## 4. Azure OpenAI driver (`azure-openai://`)

- [ ] 4.1 Write `azure_openai_test.go` (Red): URL parsing (`{resource}.cognitiveservices.azure.com/{deployment}`), endpoint path construction with `api-version` query param, `api-key` header auth, `Generate`, streaming
- [ ] 4.2 Implement `azure_openai.go` until tests pass (Green): `AzureOpenAIOpener`, `AzureOpenAIGenerator`, `AZURE_OPENAI_API_KEY` env auth, deployment-scoped URL, delegate to shared helper

## 5. Anthropic driver (`anthropic://`)

- [ ] 5.1 Write `anthropic_test.go` (Red): URL parsing, `Generate` happy path, HTTP error mapping, `Usage` mapping (input/output tokens), `Stream` over typed SSE (`content_block_delta`/`message_stop`), `Close`, required `max_tokens` defaulting
- [ ] 5.2 Implement `anthropic.go` until tests pass (Green): `AnthropicOpener`, `AnthropicGenerator`, `ANTHROPIC_API_KEY` env auth, `x-api-key` + `anthropic-version` headers, Messages API request/response, dedicated SSE consumer

## 6. Housekeeping and verification

- [ ] 6.1 Update `opener_test.go` to assert 6 registered openers (gemini, ollama, openai, compat, azure-openai, anthropic)
- [ ] 6.2 Update `example_test.go` `ExampleListOpeners` to reflect the expanded set
- [ ] 6.3 `make fmt && make vet && make test` in `go-minolas`
- [ ] 6.4 `openspec validate add-openai-anthropic-generators` then `openspec archive add-openai-anthropic-generators`
- [ ] 6.5 Update `.agents/projects/go-minolas/context/progress.md` and append RFC-3339 entry to `status.log`
