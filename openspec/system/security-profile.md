# Security Profile - go-minolas

This document defines the security boundaries, threat profile, and compliance controls for the `go-minolas` shared library.

## STRIDE Threat Matrix

| Threat Category | Threat Description | Mitigation Strategy |
| :--- | :--- | :--- |
| **Spoofing** | Masquerading connection strings in `sqlt` | Enforce explicit parsing of URLs and validate schema properties prior to passing to drivers. |
| **Tampering** | Parameter pollution in CLI argparse flags | Standardize prefix matching (`-` and `--`) and reject duplicate or unregistered argument configurations. |
| **Repudiation** | Missing log context for database initialization errors | Log errors with context parameters; avoid masking raw driver connection errors. |
| **Information Disclosure** | Leak of passwords in connection string parsing | Ensure parser strings are parsed via `url.Parse` and the credentials block is redacted in returned status structures. |
| **Denial of Service** | Infinite loops in parser loop or unbounded connection pools | Enforce timeouts on connection checks. Validate that CLI inputs do not cause buffer overflows. |
| **Elevation of Privilege** | Execution of arbitrary commands from CLI input | Sanitize console outputs and CLI inputs. Never pass unchecked input strings to shell executors. |

## Compliance Controls (C1 - C11)

### C1: Secure Transport
- `pkg/db/sqlt` must prefer encrypted connections by default. Secure flags (`encrypt=true`) must be supported in scheme options.

### C2: Rate Limiting
- Not applicable directly for library packages; limits must be enforced by integrating client applications.

### C3: Authentication & Token Security
- Connections opened via `sqlt` must accept standard credentials parameters. Redact passwords in connection string debugging representations.

### C4: Authorization
- The library does not enforce RBAC; consumers must configure database user privileges to follow the least-privilege principle.

### C5: Identifiers
- Not applicable for library utility modules.

### C6: Request Hygiene
- CLI prompts must bound input lengths to prevent memory-exhaustion attacks.

### C7: Containment & Runtime
- Ensure library functions run with standard thread safety and make no assumptions about running as root or write access to the filesystem.

### C8: Secrets Management
- Database configuration details and passwords must not be hardcoded in tests or examples.

### C9: Structured Logging
- The library must not print to stdout/stderr. All logging must be bubbled up as structured error objects to the caller.

### C10: Supply Chain & CI/CD
- Pinned and verified dependencies inside `go.sum`. Run vulnerability analysis on sub-packages.

### C11: Graceful Teardown
- Database connections returned by `sqlt` must implement the `io.Closer` interface, enabling cleanly closing pools.
