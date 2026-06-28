# Functional Specifications - go-minolas

This document defines the functional scope and contract requirements for the `go-minolas` shared library.

## Library Scope
`go-minolas` is a shared package library providing foundational blocks for CLI argument parsing, interactive terminal inputs, and database connectivity.

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
