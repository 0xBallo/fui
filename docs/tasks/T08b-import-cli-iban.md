# T08b — Import/export CLI commands and account IBAN

Branch `feat/import-cli`. Owned paths: new files in `internal/cli/` for `import`, `import undo`, `export`,
`connector list|show|test` (+ their registration in the root and the exit-code table), and — for the IBAN part —
`internal/domain/account.go`, a new migration in `internal/store/sqlite/migrations` with its sqlc queries and test,
and the matching code in `internal/app/importing`. Read `AGENTS.md` first, then the merged `internal/cli`
(`Deps`, `writeChange`, renderers, `help --json` golden, exit codes), `internal/app/importing`,
`internal/store/sqlite`.

## Deliverables

1. **CLI** (thin commands, ≤ 60 lines, no business logic, same Plan/Apply/`--dry-run`/`--yes`/agent-mode flow
   as the other write commands):
   - `fui import <file> [--connector csv|fui|<name>] [--profile <p>] [--account <name>] [--map ref=account]...
     [--create-accounts] [--since date] [--dry-run] [--yes]` — preview shows new / duplicate / doubtful / errors /
     unmapped refs / new categories / transfer proposals / balance discrepancies / already-imported flag; with a TTY
     and no `--no-input`, unmapped refs are asked interactively (huh) and saved; otherwise `unmapped_accounts`.
     `.fui.jsonl` files default to the `fui` connector.
   - `fui import undo <batch>` and `fui import list` (batches, if the store exposes them; otherwise skip list and
     say so in the report).
   - `fui export [--format csv|json|fui] [tx list filters] [--all] [--output file]` (`--all` = full dump into a
     directory, one file per entity).
   - `fui connector list|show <name>|test <name> <file>`.
   - `cli.Deps` gains the importing service; update `help --json` golden and exit-code table (`unmapped_accounts`
     → 1, `invalid_stream`/`connector_failed` → 2).
2. **Account IBAN** (your T08 request): optional `IBAN` on `domain.Account` (validated: country code + check
   digits mod-97), forward migration `0002` adding a nullable `iban` column with a migration test from a v1 DB,
   sqlc regenerated, `account add/edit --iban`, and transfer matching in the import pipeline using the IBANs of the
   user's existing accounts. Migration in its own commit with its test and generated code.

## Acceptance

- CLI tests on memstore for every new command (dry-run leaves the store unchanged, agent mode, JSON output golden).
- Store migration test v1 → v2; memstore contract still green.
- `golangci-lint run ./...` clean; `go test -race ./...` green.
