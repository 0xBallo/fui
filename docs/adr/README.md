# Architecture Decision Records (ADRs)

This directory documents the significant architectural decisions for FUI using the Markdown Any Decision Records (MADR) format.

Architectural decisions are immutable once accepted: if a decision is reconsidered, a new ADR is created that explicitly supersedes the earlier one.

## Index

| ADR | Title | Status | Date |
| --- | --- | --- | --- |
| [0001](0001-hexagonal-architecture.md) | Hexagonal architecture with pure domain and depguard enforcement | Accepted | 2026-11-11 |
| [0002](0002-integer-amounts-and-rounding.md) | Integer minor units for amounts and single-point half-away-from-zero rounding | Accepted | 2026-11-11 |
| [0003](0003-sqlc-and-forward-only-migrations.md) | sqlc instead of ORM with forward-only goose migrations and automatic backup | Accepted | 2026-11-11 |
| [0004](0004-plan-apply-pattern.md) | Plan/Apply pattern for all write operations | Accepted | 2026-11-11 |
| [0005](0005-public-proto-fui1.md) | fui/1 streaming protocol in pkg/fuiproto as the sole public package | Accepted | 2026-11-11 |
| [0006](0006-trunk-based-development.md) | Trunk-based development with Conventional Commits and rebase merges | Accepted | 2026-11-11 |
| [0007](0007-golden-file-contracts.md) | Golden files as contract for JSON output, schema, help, and connector streams | Accepted | 2026-11-11 |
| [0008](0008-xdg-paths-on-macos.md) | Consistent XDG base directory paths on Linux and macOS | Accepted | 2026-11-11 |
