# G02 — Import pipeline and preview

Branch `feat/g02-import` · worktree `../fui-wt/g02` · TL: Claude Sonnet medium · deps: none.
Read first: `docs/PLAN-gaps.md` (§2 P14; §5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum),
F: "Import, export e riconciliazione" (Flusso di import, Revolut), "Formato FUI", "Moduli" (Effetti di un modulo
disattivato), "Requisiti funzionali — TUI" (Import); T: "Connettori", "Modello dati".

## Owned paths

`internal/app/importing/` (except `dump*.go`, owned by G07), `internal/connector/csv*.go`,
`internal/store/sqlite/{migrations/<next>_import_*.sql,queries/{transactions,security*}.sql,repo_*}` only for the
columns below, `internal/domain/rule*.go` (category_hint condition), `internal/cli/import*.go`,
`internal/cli/render/import*.go`, `internal/tui/screens/importer/`, `pkg/fuiproto/` (only if a field is missing).

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a2-4 | Transfer proposals never applied (`Accept` never set) | `importing/transfers.go:192-196`, `apply.go:433` | P14: CLI `--accept-transfers` (all) and, in human interactive mode, a prompt per proposal; TUI preview lists proposals with a toggle; accepted ones become linked transfers on apply |
| a2-7, a3-18 | Records of disabled modules not offered | `importing/build.go:127-140`, `rows.go:71-73` | Preview lists them per module; CLI prompt "enable <module> / skip"; non-interactive `--enable-modules` (enables via the module use case, then applies) else skip + warning; TUI dialog. `schedule`/`installment` are only *detected* here; G03 imports them |
| a2-25 | Preview hides warnings, new categories/tags/payees, details | `cli/render/import.go:73-76`, `tui/screens/importer/preview.go:129-140` | CLI and TUI preview show: warnings, new categories/tags/payees, errors and discrepancies with line, transfer proposals, balance mismatches |
| a2-28 | `balance` records ignored for existing accounts | `importing/rows.go:487-518` | Compare with the computed balance at that date; preview shows the mismatch and suggests `fui reconcile ACCOUNT --date D --balance B` |
| a2-30 | `category_hint` unused | `pkg/fuiproto/types.go:195` | Rule condition `hint` (`equals`/`contains`, case-insensitive); import passes the hint to rules only (never applied directly) |
| a2-33 | `original` and `raw` dropped | `importing/rows.go`, `schema.sql` | Migration: nullable columns for original amount/currency and raw; stored on import, written by FUI export, shown in `tx show` |
| a3-13 | `import undo` leaves security ops | `schema.sql:107-121`, `importing/undo.go` | Migration: `security_ops.import_batch_id`; undo deletes the batch's ops with their cash tx (refuses reconciled as today) |
| a2-1 | No CSV column mapper / guided detection | `tui/screens/importer/source.go:22`, `connector/csv.go` | CSV header sniffing (delimiter, date format, decimal separator, column roles) → suggested profile; TUI mapping form (column → role, preview of 5 rows) saving an import profile; CLI `fui import detect FILE [--save NAME]` (name is fixed; ask the Architect before changing it) |

## Suggested MTs

1. store: one migration for `original`/`raw` and `security_ops.import_batch_id` + queries + migration test.
2. app: original/raw + balance-mismatch + category_hint condition (domain rule file) + undo of security ops.
3. app: transfer acceptance and disabled-module records in plan/apply (plan carries `ModuleRecords`, apply takes
   `AcceptTransfers`, `EnableModules`); contract tests incl. idempotent re-import.
4. connector: CSV detection (`fuzz` the sniffer) + app use case returning a suggested profile.
5. cli: flags, prompts (only when stdin is a TTY and not `--json`), preview render, `import detect`, testscripts.
6. tui: preview sections + toggles + module dialog; CSV mapper form; goldens and teatest flows.

## DoD

PLAN §6, plus: fuzz test for the CSV sniffer; testscript round trip export FUI → import keeps `original`/`raw`;
`import undo` e2e with a `security_op` batch; property: accepting zero proposals = today's behaviour.
