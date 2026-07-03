## ADDED Requirements

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
