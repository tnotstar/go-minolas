# Functional Specifications - go-minolas

This document defines the functional scope and contract requirements for the `go-minolas` shared library.

## Library Scope
`go-minolas` is a shared package library providing foundational blocks for CLI argument parsing, interactive terminal inputs, database connectivity, and AI text generation.

## Core Packages

### 1. `pkg/cli`
- Read interactive input from the terminal with custom verification functions.
- Display structured menus for select-one options and yes/no confirmations.

### 2. `pkg/cli/argparse`
- Provide type-safe CLI flag parsing (Bool, String, Int) utilizing Go standard library structures.
- Support nested subcommand parsing and automatic help instruction generation.

### 3. `pkg/db/sqlt`
- Open sql database connections using standard URL schemes (e.g. `sqlite://`, `sqlserver://`).
- Provide an extensible driver registration interface (`Opener`) to add custom database adapters.

### 4. `pkg/ai/generators`
- Generate text completions from pluggable AI providers through a uniform `Generator` port exposing `Generate` (one-shot) and `Stream` (chunked, channel-based) operations, plus `Close` for resource release.
- Connect to providers using a URL scheme dispatcher (`Open`) backed by a thread-safe, self-registering `Opener` registry, mirroring the pluggable pattern of `pkg/db/sqlt`.
- Configure each generation request through functional options (`WithModel`, `WithTemperature`, `WithMaxOutputTokens`, `WithTopP`, `WithTopK`, `WithSystemInstruction`, `WithStopSequences`).
- Provide built-in providers:
  - **Gemini** (`gemini://`): Google Gemini REST API (`generativelanguage.googleapis.com`), API key read from the `GEMINI_API_KEY` / `GOOGLE_API_KEY` environment variables, SSE streaming via `streamGenerateContent`.
  - **Ollama** (`ollama://`): local Ollama server (`localhost:11434`), no authentication, NDJSON streaming via `/api/generate`.
