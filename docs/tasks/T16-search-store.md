# T16 — Store search: `search_text`, FTS5 trigram, port on `domain.TxFilter`

Branch `feat/search-store` (based on `main` after T15 is merged). Owned paths: `internal/store/` (new migration
`0003_search.sql`, queries, repo code, generated sqlc, `schema.sql`), `internal/app/ports.go` (TxFilter port),
`internal/app/transaction.go` + other `internal/app` callers of the old `app.TxFilter`, `internal/app/importing/`
callers, `internal/testutil/memstore/`, `internal/cli/` and `internal/tui/` call sites only where the type change
forces it (no new flags/UI: T17/T20). Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md "Ricerca testuale", "Moduli e schema", Stack row "Ricerca testuale", open point on FTS5 in
`modernc.org/sqlite`, invariant "Per ogni TxFilter generato, la query SQL restituisce esattamente ...",
"Matrice dei moduli" (benchmark). Read merged `internal/domain/txfilter.go` and `internal/textnorm` first.

1. Verify FTS5 + `tokenize='trigram'` work in the `modernc.org/sqlite` version in go.mod (a test that creates
   the virtual table). If not available, fall back to `LIKE` on `search_text` and say so in `DEV:`.
2. Migration `0003_search.sql` (own commit, with migration test from a v2 DB with data): `transactions.search_text
   TEXT NOT NULL DEFAULT ''`, external-content FTS5 table `transactions_fts` + sync triggers, backfill. Because
   SQL cannot do NFD folding, the backfill of existing rows is done in Go right after the migration (or via a Go
   migration if goose is used that way) using `textnorm.SearchText(description, payee name, notes)`; every
   insert/update path in the store sets `search_text`.
3. Replace `app.TxFilter` with `domain.TxFilter` in the `TransactionRepo.List` port (keep keyset cursor for the
   default date sort; for `amount`/`payee` sorts paging is limit-only — document in godoc). Translate every
   filter field to SQL; text: FTS `MATCH` for ≥ 3 runes, `LIKE` on `search_text` below; normalize the query with
   `textnorm.Normalize`. `ListTxRequest`/`ResolveTxFilter` in app build a `domain.TxFilter`.
4. memstore implements List with `TxFilter.Match` + `Less` (it is the oracle).
5. Property test (rapid) on the real SQLite store: random transactions + random `TxFilter` → SQL result equals
   memstore/`Match` result in the same order. `BenchmarkSearch100k` (100 000 tx, filter + text) with the 200 ms
   target documented; it is not run in `make test`.
