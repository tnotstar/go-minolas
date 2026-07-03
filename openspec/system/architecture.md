# Technical Architecture - go-minolas

This document outlines the technical architecture of the `go-minolas` shared packages.

## Package Architecture

```mermaid
graph TD
    subgraph pkg/cli/argparse
        A[ArgumentParser] --> B[Argument Fields]
        A --> C[Subcommands]
    end
    subgraph pkg/db/sqlt
        D[Open URL] --> E[Opener Registry]
        E --> F[SQLite Opener]
        E --> G[SQLServer Opener]
    end
    subgraph pkg/ai/generators
        H[Open URL] --> I[Opener Registry]
        I --> J[Gemini Opener]
        I --> K[Ollama Opener]
        J --> L[Gemini Generator]
        K --> M[Ollama Generator]
    end
```

- **ArgumentParser:** Translates input argument string arrays into target pointers (String, Bool, Int) with default value fallbacks.
- **Opener Registry:** Thread-safe registry mapping connection schemes to custom SQL openers implementing `CanOpen` and `Open`.
- **SQLite Opener:** Connects to file-based or in-memory SQLite databases using CGo-free `modernc.org/sqlite`.
- **SQLServer Opener:** Connects to Microsoft SQL Server/Azure SQL instances using `github.com/microsoft/go-mssqldb`.
- **Generators Opener Registry:** Thread-safe registry mapping AI provider URL schemes (e.g. `gemini://`, `ollama://`) to openers implementing `CanOpen` and `Open`, mirroring the `pkg/db/sqlt` pluggable pattern.
- **Gemini Opener/Generator:** Creates a `Generator` backed by the Google Gemini REST API; reads the API key from the `GEMINI_API_KEY`/`GOOGLE_API_KEY` environment variables and streams responses via SSE.
- **Ollama Opener/Generator:** Creates a `Generator` backed by a local Ollama server; requires no authentication and streams responses as NDJSON.
