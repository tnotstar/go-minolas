# ai-generators Specification

## Purpose
TBD - created by archiving change document-existing-ai-generators. Update Purpose after archive.
## Requirements
### Requirement: Uniform text generation across providers
The `pkg/ai/generators` package SHALL expose a `Generator` interface abstracting AI text completion behind two operations — `Generate` returning a single `*Response` and `Stream` returning a read-only `<-chan StreamChunk` — plus a `Close` method for resource release. Implementations from any provider SHALL be substitutable through this interface without caller changes.

#### Scenario: One-shot generation returns a complete response
- **WHEN** a caller invokes `Generate(ctx, prompt, opts...)` on a registered provider
- **THEN** the system returns a `*Response` whose `Text` contains the completion, plus `Model`, `FinishReason`, and `Usage` (token counts) populated from the provider's reply

#### Scenario: Streaming yields chunks until completion or failure
- **WHEN** a caller invokes `Stream(ctx, prompt, opts...)` on a registered provider
- **THEN** the system returns a channel that emits `StreamChunk` values as content arrives, closes the channel when the stream ends, and signals a mid-stream failure by sending a `StreamChunk` with a non-nil `Error` before closing

### Requirement: Pluggable provider registry dispatched by URL scheme
The package SHALL select a concrete provider at runtime via `Open(ctx, aiurl)`, which parses the URL and delegates to the first `Opener` whose `CanOpen` matches the URL scheme. Providers register themselves through a thread-safe `RegisterOpener` call (typically from an `init` function) and duplicate registrations SHALL panic.

#### Scenario: Registered opener is selected by scheme
- **WHEN** `Open` is called with a URL whose scheme matches a registered opener (e.g. `gemini://`, `ollama://`)
- **THEN** the system delegates to that opener and returns its `Generator`

#### Scenario: Unregistered scheme is rejected
- **WHEN** `Open` is called with a URL scheme no registered opener can handle
- **THEN** the system returns `ErrUnsupportedOpener` without contacting any network endpoint

### Requirement: Functional configuration options
Each generation request SHALL be configured through functional options (`WithModel`, `WithTemperature`, `WithMaxOutputTokens`, `WithTopP`, `WithTopK`, `WithSystemInstruction`, `WithStopSequences`) applied to a zero-value `Config`. Zero-value fields SHALL mean "use the provider's default".

#### Scenario: Option overrides provider default
- **WHEN** a caller passes `WithModel("gemini-2.0-pro")` to a request
- **THEN** the provider uses that model for the request instead of its compiled default

#### Scenario: Omitted option falls back to default
- **WHEN** a caller omits the `WithTemperature` option
- **THEN** the provider applies its own default sampling temperature for the request

### Requirement: Gemini provider via REST and SSE
The package SHALL ship a `gemini://` opener backed by the Google Gemini REST API at `generativelanguage.googleapis.com`, reading the API key exclusively from the `GEMINI_API_KEY` environment variable with `GOOGLE_API_KEY` as fallback, authenticating via the `x-goog-api-key` header, and streaming via the `streamGenerateContent` endpoint using SSE.

#### Scenario: Gemini API key resolution from environment
- **WHEN** a `gemini://` URL is opened and `GEMINI_API_KEY` is set
- **THEN** the resulting generator authenticates requests using that key, regardless of `GOOGLE_API_KEY`

#### Scenario: Gemini streaming over SSE
- **WHEN** `Stream` is invoked on a Gemini generator
- **THEN** response chunks are parsed from `data: `-prefixed SSE lines and emitted on the channel until a `[DONE]` sentinel or stream end

### Requirement: Ollama provider via REST and NDJSON
The package SHALL ship an `ollama://` opener backed by a local Ollama server at `localhost:11434`, requiring no authentication, and streaming responses as newline-delimited JSON (NDJSON) via `/api/generate`.

#### Scenario: Ollama runs unauthenticated against a local server
- **WHEN** a caller opens `ollama://` with no credentials in the URL or environment
- **THEN** the resulting generator issues requests to the local server without authentication headers

#### Scenario: Ollama streaming over NDJSON
- **WHEN** `Stream` is invoked on an Ollama generator
- **THEN** each newline-delimited JSON object yields a `StreamChunk` from its `response` field, and the stream ends when an object with `done: true` is received

### Requirement: OpenAI provider via Chat Completions
The package SHALL ship an `openai://` opener backed by the OpenAI Chat Completions API at `api.openai.com`, reading the API key exclusively from the `OPENAI_API_KEY` environment variable, authenticating via the `Authorization: Bearer <key>` header, and streaming responses as SSE with a `data: [DONE]` terminator.

#### Scenario: OpenAI defaults to api.openai.com and gpt-4o-mini
- **WHEN** a caller opens `openai://` with no host or model
- **THEN** the resulting generator targets `https://api.openai.com` and uses the `gpt-4o-mini` model unless overridden by `WithModel`

#### Scenario: OpenAI one-shot generation
- **WHEN** `Generate` is invoked on an OpenAI generator with a prompt
- **THEN** the system posts a Chat Completions request and returns a `*Response` whose `Text` is `choices[0].message.content`, with `Usage` populated from the `usage` object

#### Scenario: OpenAI streaming over SSE
- **WHEN** `Stream` is invoked on an OpenAI generator
- **THEN** the system parses `data: `-prefixed SSE lines, emits `choices[0].delta.content` as `StreamChunk` values, and ends the channel at the `data: [DONE]` sentinel

### Requirement: Compat provider for OpenAI-format endpoints
The package SHALL ship a `compat://` opener for any OpenAI Chat Completions-format endpoint whose host differs from `api.openai.com` (e.g. OpenRouter, vLLM, LM Studio, Together, Groq, DeepSeek). The host and model SHALL be required in the URL; opening a `compat://` URL without a host or without a model SHALL return an error. Authentication SHALL fall back from `COMPAT_API_KEY` to `OPENAI_API_KEY`.

#### Scenario: Compat requires explicit host and model
- **WHEN** a caller opens `compat://` with an empty host or an empty model
- **THEN** the opener returns an error and no generator is created

#### Scenario: Compat API key fallback
- **WHEN** `COMPAT_API_KEY` is unset and `OPENAI_API_KEY` is set
- **THEN** the resulting compat generator authenticates using the `OPENAI_API_KEY` value

### Requirement: Azure OpenAI provider via deployment-scoped Chat Completions
The package SHALL ship an `azure-openai://` opener backed by Azure OpenAI, using the URL format `azure-openai://{resource}.cognitiveservices.azure.com/{deployment}`, authenticating via the `api-key` header with the `AZURE_OPENAI_API_KEY` environment variable, and targeting `/openai/deployments/{deployment}/chat/completions?api-version=2024-10-21`. Streaming SHALL use SSE identical to the OpenAI provider.

#### Scenario: Azure OpenAI endpoint construction
- **WHEN** a caller opens `azure-openai://my-resource.cognitiveservices.azure.com/my-deployment`
- **THEN** the resulting generator issues requests to `https://my-resource.cognitiveservices.azure.com/openai/deployments/my-deployment/chat/completions?api-version=2024-10-21`

#### Scenario: Azure OpenAI authentication
- **WHEN** `AZURE_OPENAI_API_KEY` is set in the environment
- **THEN** the resulting generator sends the `api-key` request header with that value and no `Authorization` header

### Requirement: Anthropic provider via Messages API
The package SHALL ship an `anthropic://` opener backed by the Anthropic Messages API at `api.anthropic.com`, reading the API key exclusively from the `ANTHROPIC_API_KEY` environment variable, authenticating via the `x-api-key` header together with `anthropic-version: 2023-06-01`, targeting `/v1/messages`, and streaming via typed SSE events. Because Anthropic mandates `max_tokens`, the provider SHALL default it to 1024 when `WithMaxOutputTokens` is not supplied.

#### Scenario: Anthropic defaults to claude-3-5-sonnet-latest
- **WHEN** a caller opens `anthropic://` with no model
- **THEN** the resulting generator uses `claude-3-5-sonnet-latest` unless overridden by `WithModel`

#### Scenario: Anthropic max_tokens default
- **WHEN** `Generate` is invoked without `WithMaxOutputTokens`
- **THEN** the request body includes `max_tokens: 1024`

#### Scenario: Anthropic streaming over typed SSE
- **WHEN** `Stream` is invoked on an Anthropic generator
- **THEN** the system emits `StreamChunk` values from `event: content_block_delta` payloads and closes the channel after `event: message_stop`

### Requirement: Shared Chat Completions wire-format helper
The `openai`, `compat`, and `azure-openai` providers SHALL share a single unexported Chat Completions helper for request construction, response parsing, and SSE consumption, so that the OpenAI Chat Completions wire format is defined in exactly one place. Each provider SHALL retain its own `Opener` and `Generator` types with distinct URL-scheme dispatch and authentication.

#### Scenario: OpenAI-format providers share response parsing
- **WHEN** any of `openai`, `compat`, or `azure-openai` receives a Chat Completions response
- **THEN** the response is parsed by the same shared helper into a `*Response` with `Text`, `Model`, `FinishReason`, and `Usage`

