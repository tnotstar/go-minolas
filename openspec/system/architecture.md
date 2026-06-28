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
```

- **ArgumentParser:** Translates input argument string arrays into target pointers (String, Bool, Int) with default value fallbacks.
- **Opener Registry:** Thread-safe registry mapping connection schemes to custom SQL openers implementing `CanOpen` and `Open`.
- **SQLite Opener:** Connects to file-based or in-memory SQLite databases using CGo-free `modernc.org/sqlite`.
- **SQLServer Opener:** Connects to Microsoft SQL Server/Azure SQL instances using `github.com/microsoft/go-mssqldb`.
