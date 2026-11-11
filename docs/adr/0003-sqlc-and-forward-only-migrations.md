# ADR 0003: sqlc instead of ORM with forward-only goose migrations and automatic backup

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

Database access in Go is frequently managed either through heavy object-relational mapping (ORM) libraries or ad-hoc SQL strings. ORMs introduce complex query generation, hidden N+1 performance pitfalls, and runtime reflection errors. At the same time, bidirectional database schema migrations (up/down) in production databases frequently risk partial rollbacks, schema drift, and irreversible data loss when a down-migration attempts to reconstruct dropped columns or transformed data.

## Considered Options

1. **Full-featured ORM (e.g. GORM, Ent):** High developer convenience initially, but opaque query plans, reflection overhead, and leaky abstractions.
2. **Handcrafted `database/sql` queries:** Full query control, but repetitive boilerplate and risk of runtime column/parameter mismatch errors.
3. **`sqlc` with forward-only `goose` migrations and automatic pre-migration backups:** SQL queries validated and compiled into type-safe Go code at build time; schema migrations only roll forward, while rollback is achieved by restoring a pre-migration backup.

## Decision Outcome

Chosen option: **`sqlc` with forward-only `goose` migrations and automatic backup**, because:
- `sqlc` generates idiomatic, zero-allocation Go code directly from raw SQL, validating query syntax and types against the SQLite schema during compilation.
- Migrations are embedded directly in the binary (`embed.FS`) and managed with `goose` applying only forward migrations.
- Down migrations are banished in production: before applying any schema migration, FUI automatically executes a full database backup (`fui-YYYYMMDD-HHMMSS-migrate.db`).
- Rolling back a failed migration is accomplished safely by restoring the verified pre-migration backup archive.

### Consequences

* **Positive:**
  - Type-safe, compile-time verified database interactions without reflection overhead.
  - Complete visibility into actual SQL statements and indexing strategies.
  - Zero risk of faulty down-migration scripts destroying historical user data.
* **Negative:**
  - Developers must write SQL schemas and queries manually.
  - Requires running code generation (`make generate`) and committing generated artifacts whenever SQL queries change.
