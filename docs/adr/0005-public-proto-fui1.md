# ADR 0005: fui/1 streaming protocol in pkg/fuiproto as the sole public package

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

Financial institutions export data in an endless variety of formats (CSV, XLSX, PDF, OFX, CAMT). Supporting every bank natively inside the main FUI binary would bloat the codebase, introduce fragile dependencies (such as PDF or spreadsheet parsers), and increase the attack surface. FUI needs an extensible connector system that enables bank-specific parsers to run as separate processes while retaining strict isolation and interface stability.

## Considered Options

1. **In-process Go plugins (`plugin` package):** Difficult to build, platform-dependent, and prone to runtime panics across compiler versions.
2. **RPC network services (gRPC, net/rpc):** Violates the local-first rule by introducing network ports and sockets, requiring complex port negotiation and daemon management.
3. **Streaming JSON Lines protocol (`fui/1`) over standard I/O with `pkg/fuiproto`:** External connector executables (`fui-connector-*`) communicating over standard input/output using a streaming JSON Lines protocol defined in a dedicated public package.

## Decision Outcome

Chosen option: **Streaming JSON Lines protocol (`fui/1`) over standard I/O with `pkg/fuiproto`**, because:
- `pkg/fuiproto` is the single public package in the FUI repository, containing message types and conformance validation without importing internal application packages.
- Connectors run as separate standalone processes, reading input files and streaming standardized records (`meta`, `account`, `tx`, `end`) to `stdout` with line-by-line validation.
- FUI's core remains strictly offline and secure: external connectors have no direct access to the SQLite database or configuration secrets.
- Connectors can be implemented in any programming language (Go, Python, Rust, shell) or tested in isolation via conformance suites.

### Consequences

* **Positive:**
  - Robust isolation between untrusted parser logic and core accounting state.
  - Universal connector compatibility across languages without network ports.
  - Public contract is versioned (`fui/1`) and thoroughly tested for backward compatibility.
* **Negative:**
  - Serialization and deserialization overhead over OS pipes (negligible for typical statement sizes).
  - Requires maintaining a stream validation state machine inside FUI.
