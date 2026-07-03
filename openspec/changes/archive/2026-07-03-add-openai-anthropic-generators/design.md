# Design — add-openai-anthropic-generators

## Context

`pkg/ai/generators` already implements the `Generator` port and a pluggable `Opener` registry, with Gemini and Ollama providers shipping today (see `openspec/specs/ai-generators/spec.md`). Each provider is a self-contained file: a `*Opener` struct (`Id`/`CanOpen`/`Open`) that self-registers in `init()`, a `*Generator` struct (`Generate`/`Stream`/`Close`), private JSON request/response types, and a `parseProviderURL`-based constructor. We extend this pattern with four new providers and one shared helper.

All providers use only the Go standard library and follow the established conventions: `http.NewRequestWithContext` (caller-supplied `context.Context`), `https://` endpoints, environment-variable authentication, and functional options from the existing `Config`.

## Goals / Non-Goals

**Goals**
- Four new providers reachable via URL scheme: `openai://`, `compat://`, `azure-openai://`, `anthropic://`.
- Each provider registers through the existing `RegisterOpener`; no changes to the port or registry.
- TDD coverage parity with Gemini/Ollama (URL parsing, happy-path `Generate`, HTTP error mapping, `Usage` mapping, `Stream`, `Close`).
- Zero new third-party dependencies.

**Non-Goals**
- Multimodal content (images, audio, tool/function calling). Out of scope; the port is text-in/text-out.
- The Phase-2 OpenAI-compatibility **server** adapter (`pkg/ai/openaicomp/`). Tracked separately.
- Tokenizer implementations. Token counts come from provider responses only.

## Decisions

### D1: Chat Completions as the OpenAI wire format
Chat Completions (`/v1/chat/completions`, `messages` array, `roles` `system`/`user`/`assistant`) is the modern, broadly-supported OpenAI surface — it is what every compatible host (Azure, OpenRouter, vLLM, LM Studio, Together, Groq, DeepSeek) implements. The legacy `/v1/completions` prompt API is deprecated and has poor system-prompt semantics.
- **Alternatives considered:** legacy Completions (rejected: deprecated, poor system role).

### D2: A shared, unexported Chat Completions helper
`openai`, `compat`, and `azure_openai` share the exact same request shape, response shape, and SSE streaming format. Duplicating ~150 LOC of JSON marshalling/response parsing across three files would violate DRY and make wire-format fixes error-prone.
- **Decision:** introduce `chat_completion.go` with unexported helpers `buildChatCompletionRequest(cfg, prompt) chatCompletionRequest`, `parseChatCompletionResponse(body, model) *Response`, and `consumeChatCompletionSSE(ctx, body, ch)`. Each provider keeps its own `Opener`/`Generator` structs (distinct URL schemes, auth headers, endpoint construction) but delegates the wire-format work to these helpers.
- **Alternative considered:** an embedded `chatCompletionClient` struct (rejected: it would couple provider identity to an implementation detail and complicate `Open`'s responsibility split). Plain function helpers keep each provider self-contained.

### D3: Provider identity via URL scheme, not query params
Each provider has its own URL scheme (`openai://`, `compat://`, `azure-openai://`, `anthropic://`). This matches the Gemini/Ollama precedent and keeps `CanOpen` a trivial scheme equality check.
- **Alternatives considered:** a single `openai://` scheme with `?host=azure` query param (rejected: pushes dispatch logic into each opener and muddies `CanOpen`).

### D4: Authentication sources
| Provider | Auth | Header | Env var(s) |
|---|---|---|---|
| `openai` | Bearer token | `Authorization: Bearer <key>` | `OPENAI_API_KEY` |
| `compat` | Bearer token | `Authorization: Bearer <key>` | `COMPAT_API_KEY`, fallback `OPENAI_API_KEY` |
| `azure_openai` | Subscription key | `api-key: <key>` | `AZURE_OPENAI_API_KEY` |
| `anthropic` | API key | `x-api-key: <key>` + `anthropic-version: 2023-06-01` | `ANTHROPIC_API_KEY` |

All keys come exclusively from environment variables — consistent with the existing Gemini provider and the ADR 4 directive (no secrets in URLs/logs). URLs never carry API keys.

### D5: Azure OpenAI URL layout
Azure is deployment-scoped, not model-scoped. The URL format is `azure-openai://{resource}.cognitiveservices.azure.com/{deployment}` and the request path is `/openai/deployments/{deployment}/chat/completions?api-version=2024-10-21`. The default API version is a compiled constant (`defaultAzureOpenAIAPIVersion = "2024-10-21"`); the last path segment of the URL is the deployment name.

### D6: Streaming transports
- OpenAI/compat/Azure: SSE with `data: {"choices":[{"delta":{"content":"..."}}]}` lines, terminator `data: [DONE]` — handled by the shared `consumeChatCompletionSSE`.
- Anthropic: SSE with typed `event:` lines. Content arrives on `event: content_block_delta` (with `data: {"delta":{"text":"..."}}`); the stream ends on `event: message_stop`. A dedicated parser in `anthropic.go` handles the richer event model.

### D7: Option-to-wire mapping
- `Config.SystemInstruction` → OpenAI/compat/Azure: a leading `{"role":"system"}` message; Anthropic: the top-level `system` field.
- `Config.TopK` → Anthropic `top_k` and (existing) Gemini `topK`/Ollama `top_k`. OpenAI/compat/Azure have no `top_k`; the option is silently ignored there (the OpenAI Chat Completions API does not accept it).
- All other options map 1:1 (`temperature`, `max_tokens`/`max_tokens`, `top_p`, `stop`).

## Risks / Trade-offs

- **[Wire-format drift]** OpenAI/Azure/Anthropic may evolve their JSON shapes. → Each provider owns its private JSON structs and the shared helper is the single seam for Chat Completions; a format change is a one-file edit. No backwards-compatibility shim is offered (callers consume the stable `Response`/`StreamChunk` types).
- **[Azure API versioning]** Azure's `api-version` changes over time. → Pinned to a compiled constant with a clear `// update when Azure deprecates` comment; overridable later via a functional option if needed (deferred — non-goal here).
- **[Anthropic max_tokens is required]** Unlike OpenAI, Anthropic mandates `max_tokens` per request. → The Anthropic provider supplies a sane default (`defaultAnthropicMaxTokens = 1024`) when `WithMaxOutputTokens` is not set, documented in the spec.
- **[compat provider ambiguity]** Without a canonical host, a typo'd `compat://` URL fails at request time, not at `Open`. → `compat`'s `Open` requires a non-empty host and model in the URL and returns an explicit error otherwise (unlike `openai` which supplies defaults).

## Migration Plan

Additive only — no existing behavior changes. Consumers auto-pick up the new openers by importing the package (each registers in `init()`). Rollback = remove the new files; no callsite changes required.

## Open Questions

_None material. Minor: whether to later add `WithAzureAPIVersion`/`WithEndpoint` functional options — deferred to a follow-up change if real consumers need it.
