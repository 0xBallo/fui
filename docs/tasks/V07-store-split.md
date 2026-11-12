# V07 — Store: tags, payees, splits
Branch `feat/v-store-split`. Owned: store migration+queries+`repo_{tags,payees,splits}*.go`, app ports for them, memstore files `{tags,payees,splits}*.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). Store layer: forward-only goose migration in its own commit with a migration test that runs it on a populated archive at the previous version; sqlc queries in `internal/store/sqlite/queries/`, regenerate with `go generate ./internal/store/...` (commit generated `sqlc/` + `schema.sql` + `testdata/schema.golden`); repo implementing a new app port (add the port interface to `internal/app/ports.go` and to `Repos`; implement it in `internal/testutil/memstore` too, plus the shared contract test). Read the merged domain types first. Migration number: take the next free number on `main` when you start; if another store task merged first, renumber on rebase. Coverage of `internal/store/sqlite` ≥ 80%. Plan: `docs/PLAN-v0.2.md` §2–4. Spec: tecniche.md "Modello dati e schema SQLite", "Persistenza e migrazioni".

1. Tables `tags` (name UNIQUE NOCASE), `transaction_tags`, `payees` + `payee_aliases`, FK `transactions.payee_id → payees`, `transaction_splits` (tx, category, amount, note; ON DELETE CASCADE from tx, RESTRICT on category).
2. Repos CRUD + `TagsOf(tx)`, `SplitsOf(tx)`, `ReplaceSplits(tx, rows)` validated with `domain.ValidateSplits`.
3. `search_text` must include the payee name now (update the store insert/update paths and the FTS backfill if needed) and `TxFilter` payee filter must work end to end; extend the SQL-vs-Match property test with payees. Tag filter is added later by V13 (leave a note).
4. `reset --keep-settings` keeps tags and payees (extend the reset test).
