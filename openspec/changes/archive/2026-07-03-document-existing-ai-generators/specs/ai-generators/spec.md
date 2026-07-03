## ADDED Requirements

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
