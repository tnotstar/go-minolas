## Why

The `pkg/ai/generators` package already ships a working AI-generation abstraction — the `Generator` port, the pluggable `Opener` registry, and two concrete providers (Gemini, Ollama) — but this capability is absent from `openspec/system/` and from the private agent context (`sot.md`, `progress.md`). The SDD invariant in `AGENTS.md` ("no code modifications without prior specification synchronization") is currently violated for this package. We close that gap before adding any new providers (see `add-openai-anthropic-generators`).

## What Changes

- Document the existing `pkg/ai/generators` capability in `openspec/system/system-spec.md` (Generator port, Opener registry, functional options, Gemini + Ollama providers).
- Add the corresponding subgraph to `openspec/system/architecture.md`.
- Sync the private agent context: structural layout and package list in `.agents/projects/go-minolas/context/sot.md`, plus a progress checklist update in `progress.md`.
- No code changes. No new capabilities. No requirement deltas — this is purely a backfill of documentation for already-shipped behavior.

## Capabilities

### New Capabilities

- `ai-generators`: Pluggable AI text-generation capability — the `Generator` port (one-shot `Generate` + channel-based `Stream` + `Close`), a thread-safe self-registering `Opener` registry dispatched by URL scheme, functional request options, and the built-in Gemini and Ollama providers. Already implemented in `pkg/ai/generators`; this change formally captures its behavior as a spec under `openspec/specs/`.

### Modified Capabilities

_None._ `openspec/specs/` is currently empty; there is no prior spec to delta against.

## Impact

- **Specs:** `openspec/system/system-spec.md`, `openspec/system/architecture.md`.
- **Agent context:** `.agents/projects/go-minolas/context/sot.md`, `.agents/projects/go-minolas/context/progress.md`, `.agents/projects/go-minolas/context/status.log`.
- **Code:** none.
- **Dependencies:** none.
