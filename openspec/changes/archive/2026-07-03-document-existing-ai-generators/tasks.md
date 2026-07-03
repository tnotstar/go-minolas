# Tasks - document-existing-ai-generators

> Retrospective documentation change. No code. Mark items as you complete them.

## 1. System specs (public)

- [x] 1.1 Add `pkg/ai/generators` capability section to `openspec/system/system-spec.md` (Generator port, Opener registry, functional options, Gemini + Ollama providers)
- [x] 1.2 Add `pkg/ai/generators` subgraph to the mermaid diagram in `openspec/system/architecture.md`

## 2. Private agent context

- [x] 2.1 Add `pkg/ai/` to the structural layout and package description in `.agents/projects/go-minolas/context/sot.md`
- [x] 2.2 Mark AI providers as implemented in `.agents/projects/go-minolas/context/progress.md` and add an audit line
- [x] 2.3 Append RFC-3339 entry to `.agents/projects/go-minolas/context/status.log`

## 3. Verification

- [ ] 3.1 `openspec validate document-existing-ai-generators`
- [ ] 3.2 `openspec archive document-existing-ai-generators`
