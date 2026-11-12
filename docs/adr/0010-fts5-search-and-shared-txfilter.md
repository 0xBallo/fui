# ADR 0010: FTS5 trigram search on normalized text with one shared TxFilter

* Status: Accepted
* Date: 2026-11-12

## Context and Problem Statement

Users need transaction search that is partial-match, case-insensitive, and accent-insensitive, and
the same filtering semantics must apply in CLI (`tx list`, `tx search`, `export`), TUI, reports,
and store queries. Keeping separate filter implementations risks mismatches and regressions.

## Considered Options

1. **`LIKE`-only text search and separate per-surface filters:** easy to implement but slower at
   scale and semantically divergent between components.
2. **App-level full scan with in-memory filtering:** simplest correctness model, but unacceptable
   for larger archives and paging.
3. **SQLite FTS5 trigram index over normalized `search_text`, plus one domain `TxFilter` used by
   all surfaces and translated once in store SQL:** fast and consistent.

## Decision Outcome

Chosen option: **FTS5 trigram over normalized `search_text` with one shared `TxFilter`**, because:
- Normalization in Go (lowercase, diacritics removal, whitespace normalization) is shared by
  indexed text and query text, preventing index/query drift.
- FTS5 trigram enables partial matching efficiently; short queries can still fall back to `LIKE`
  on the same normalized column.
- A single `domain.TxFilter` contract drives CLI, TUI, export, and report filtering, with
  `Match(tx)` as in-memory oracle for SQL translation tests.

### Consequences

* **Positive:**
  - Unified filter behavior across product surfaces.
  - Search performance scales to large archives with indexed matching.
  - Property tests can enforce SQL results equal oracle results for generated filters.
* **Negative:**
  - Requires FTS5 trigram availability in the SQLite build; fallback must be explicit if absent.
  - SQL translation complexity grows as filter fields evolve.
