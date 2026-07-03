## Why

`pkg/ai/generators` currently ships only Gemini and Ollama. The most widely deployed AI APIs — OpenAI (and its many format-compatible hosts: Azure OpenAI, OpenRouter, vLLM, LM Studio, Together, Groq, DeepSeek, …) and Anthropic — cannot be reached through the existing `Opener` registry. Adding them makes the package a genuine universal client and unblocks downstream `go-minolas` consumers that need provider-agnostic text generation.

## What Changes

- Add an **OpenAI** provider (`openai://`) targeting the canonical `api.openai.com` Chat Completions API, authenticated via the `OPENAI_API_KEY` environment variable, with SSE streaming.
- Add a **compat** provider (`compat://`) for any OpenAI Chat Completions-format endpoint with a custom host/path (OpenRouter, vLLM, LM Studio, Together, Groq, DeepSeek, …). Host and model are required in the URL; auth falls back from `OPENAI_API_KEY` to `COMPAT_API_KEY`.
- Add an **Azure OpenAI** provider (`azure-openai://`) using Azure's deployment-scoped URL layout and `api-key` header authentication, with the API version as a query parameter.
- Add an **Anthropic** provider (`anthropic://`) targeting the Messages API (`api.anthropic.com/v1/messages`), authenticated via the `ANTHROPIC_API_KEY` environment variable (with `x-api-key` + `anthropic-version` headers), with SSE streaming.
- Introduce a small **unexported Chat Completions helper** (`chat_completion.go`) shared by `openai`, `compat`, and `azure_openai` to build requests and parse responses/SSE without duplicating wire-format logic across three files.
- No changes to the existing `Generator` port, `Opener` registry, `Config`, or to the Gemini/Ollama providers. New providers register through the existing `RegisterOpener` in their `init()`.

## Capabilities

### New Capabilities

_None at the capability folder level._ The `ai-generators` capability already exists (`openspec/specs/ai-generators/spec.md`, formalized by `document-existing-ai-generators`). This change extends it with new provider requirements.

### Modified Capabilities

- `ai-generators`: Add requirements covering the four new providers (OpenAI, compat, Azure OpenAI, Anthropic) — URL schemes, authentication sources, endpoints, streaming transports, and the shared Chat Completions helper.

## Impact

- **Code:** new files `pkg/ai/generators/openai.go`, `compat.go`, `azure_openai.go`, `anthropic.go`, `chat_completion.go` plus matching `*_test.go`. Minor updates to `opener_test.go` and `example_test.go` to assert the expanded opener set.
- **Specs:** delta against `openspec/specs/ai-generators/spec.md`.
- **Dependencies:** none new — only Go standard library (`net/http`, `net/url`, `encoding/json`, `context`, `bufio`, `os`).
- **Public API:** additive only. Existing callers are unaffected.
