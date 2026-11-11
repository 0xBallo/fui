# T04 — App ports and core use cases (Plan/Apply)

Branch `feat/app-core`. Owned paths: `internal/app/` (root package only — `internal/app/reporting/` is T05,
`internal/app/importing/` is T08) and `internal/testutil/memstore/`. Read `AGENTS.md` first, then the merged domain
package `internal/domain` (reuse its types, errors, `Resolve`, parsers — do not duplicate them).
Spec refs: technical spec "Architettura", "CLI" (Plan/Apply, errors, name references), "Persistenza" (Unit of Work),
"Pipeline di import"; functional spec "Modello di dominio", "Requisiti funzionali — CLI", MVP scope.

Other agents will build in parallel on top of your ports: the SQLite store (T07), the import pipeline (T08), the CLI
(T09) and the TUI (T10). **Your exported API is their contract**: keep it small, documented and stable, and commit the
ports first (one early commit with `ports.go`) so the shape is visible.

## Deliverables

1. **Ports** (`internal/app/ports.go`), consumer-defined:
   - `Store` with `Read(ctx, func(Repos) error) error` and `WithTx(ctx, func(Repos) error) error` (Unit of Work:
     one SQL transaction per write use case; repositories never open transactions).
   - `Repos` giving access to small repository interfaces for: accounts, transactions (incl. keyset pagination on
     `(date, id)` for lists of 100k rows, balance sum up to a date, lookups by external id / fingerprint / same
     account+amount within ±N days for dedup), categories (incl. reassign on merge/remove), transfer groups,
     fx rates (rate on or before a date), import batches + connector account refs (`connector`,`ref` → account) for
     the import pipeline and `import undo`. Model them on what the use cases (yours and T08's pipeline steps in the
     technical spec) need — nothing more.
   - `Clock` (`Now() time.Time`) and `Backuper` (`Backup(ctx, reason) (path, error)`, `Restore(ctx, path) error`).
2. **Plan/Apply** — every write use case has `PlanX(ctx, req) (XPlan, error)` that computes changes without
   writing, and `ApplyX(ctx, plan) (result, error)` that applies them in one `WithTx`. A plan is plain data that the
   CLI can render for `--dry-run` (list of changes: op create/update/delete, entity, human summary, structured
   fields) and carries `Bulk bool` for mass changes (CLI asks confirmation / requires `--yes`). Bulk writes and
   restore call `Backuper.Backup` with the right reason before applying.
3. **Use cases (MVP)**:
   - Init: create default categories (idempotent).
   - Accounts: add, list (with balance at a date, archived hidden unless requested), show, edit, archive.
   - Transactions: add (account/category by name via `domain.Resolve`, default date today from Clock in local zone,
     default status booked), list with filters (period, accounts, categories, status, text, amount range) and keyset
     pagination, show, edit, remove; editing/removing a reconciled transaction fails with `ErrReconciled` unless the
     request explicitly confirms it.
   - Transfer: two linked legs of opposite sign + optional fee; cross-currency needs an explicit destination amount
     or an fx rate for the date.
   - Categories: add, list, rename, merge (reassign txs then delete), remove (only if unused, otherwise require a
     reassign target).
   - FX rates: set (currency, date, rate as decimal text — add a `ParseRate` in app if the domain lacks it, or
     request it), list, lookup; net worth = sum of account balances converted to the base currency at the rate valid
     at the date; a missing rate is a coded error listing the currency.
   - Backup / restore: thin use cases over `Backuper`.
4. **In-memory store** `internal/testutil/memstore` implementing `Store`/`Repos` faithfully (transaction semantics:
   a failing `WithTx` leaves no changes) so app, CLI and TUI can be tested without SQLite. The SQLite store (T07)
   will run the same contract tests: put reusable repository contract tests in `memstore` as an exported
   `RunContract(t, newStore func(t) app.Store)` helper.
5. Errors: use `domain.Error` codes; add app-level codes only if needed (e.g. `confirmation_required`,
   `rate_missing`) and list them in the domain codes list via a request if they must live there.

## Acceptance

- Unit tests for every use case on memstore (table-driven), plus property tests: balance at a date independent of
  insertion order; a transfer without fee leaves net worth unchanged; Plan never writes (store unchanged after Plan).
- Coverage of `internal/app` ≥ 85 %. `golangci-lint run ./internal/app/... ./internal/testutil/...` clean.
- No business logic expected in CLI/TUI later: anything they would need to compute belongs here.
