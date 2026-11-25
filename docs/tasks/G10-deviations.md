### PR #68 feat(import): close the import pipeline and preview gaps
## Deviations
- `domain.Transaction` gains `Original`, `Raw`; `domain.SecurityOp` gains `ImportBatchID`; `store.go`, schema golden, memstore, `app/ports.go` touched for them.
- E2E scripts live in `testdata/e2e` (repo convention), not `cmd/fui/testdata`.
- TUI keys to approve: `tab` rows/details, `t` accept transfer, `m` modules, `ctrl+p` CSV mapper.
- `go.mod` untouched.
## Known gaps
- `BalanceMismatches` not recomputed on `SetInclude`.
- No-header CSVs get "column N" names plus a warning; date-time cells not detected; mapper form not scrollable for very wide CSVs.
- Rule add/edit CLI/TUI does not expose `hint_*` conditions yet.
- `schedule`/`installment` only detected (G03 imports them).
- `internal/cli/render` own-package coverage 9.2% (pre-existing, exercised through `internal/cli` and e2e).
### PR #69 feat(budget): complete the budget module with annual periods, thresholds and TUI forms
## Deviations / known gaps
- TUI edit form cannot prefill categories/tags (the port exposes names only); an empty picker keeps the current targets.
- Copy skips yearly budgets (no monthly amounts) without a plan note.
- The old 90% "‼" tier icon was dropped: tiers are now OK / WARN / OVER from the budget's own thresholds (OVER stays a text label).
- Yearly cap/envelope values are "as of the shown month".
### PR #70 feat(cli): register recurrences at startup, add reconcile ticks and tx payee/tags
## Deviations
- `recurring edit/rm` take the recurrence ID, not a name (recurrences have no name; same as `skip`/`confirm`).
- `rule test` without account/amount: `rules.Service.Test` now takes them as optional (`TestRequest.AmountMinor` is a pointer, `TestResult.NotEvaluated` lists what was skipped); the CLI notes it on stderr.
- `recurring rm` does not require `--yes` (same as `rule rm`).
- `S` on a pending recurring row discards that pending transaction; loan pending rows show "not supported yet".
- Tags in `tx edit` use `--untag` to remove.
## Known gaps
- Loan pending confirm/skip in the TUI is wired for confirm only; G03 adds `ConfirmPending`.
- Other screens' async messages do not implement `screens.Targeted` yet (only Transactions and Settings sections).
- Undo does not remove implicitly created payees/tags (G05).
- Unsaved tick toggles in the Accounts reconcile view are discarded on esc.
- `cmd/fui` and `internal/cli/render` package-level coverage is below 80% (pre-existing); total is above.
### PR #71 feat(import): import and export loans and match pending installments
## Deviations (for G10 to update F "Formato FUI")
- `schedule` record gains optional fields `debit_account`, `kind`, `principal`, `disbursed_on`, `term_months` (still fui/1, validator accepts their absence; export always writes them). Without them import only updates an existing loan matched by account, otherwise warns and skips that loan's records. (Architect decision Q1=a.)
- `fui import` never passed the enabled modules to the plan, so loan records and installment/recurrence matching were skipped; now set from config (`internal/cli/import.go`).
- `lending.PendingInstallment` renamed to `PlannedInstallment` (name clash with the new `PendingInstallment` kind).
- Testscripts live in `testdata/e2e`.
## Known gaps
- Loans without a stored schedule are not exported.
- Imported versions are validated against the version in force at `valid_from` only (no list-versions port); a later-dated existing version is not checked.
- `cmd/fui` package coverage is 59.7% (testscripts cover the CLI paths).
- No migration added (0019 stays free).
### PR #72 feat(app): reports filters, grouping, forecast horizon and net worth fixes
## Deviations
- `Service.ResolveTxFilter`/`tx list --payee` now resolve payees by name/alias (was "not supported yet").
- TUI report screen: Enter drills (run moved to `r`); the old transactions-only export (`e`, incl. `fui` format) was replaced by result export.
- Saved v1 spending/cash-flow keep the old result shape; only v2 returns grouped results.
- Spending counts outflows only (refunds not netted), as before.
## Known gaps
- No TUI UI to choose group-by/filters; transactions filter form drops month/year/tag/payee when edited after a drill-down.
- `report compare --chart` is accepted but has no visual effect yet.
- `internal/cli/render` package coverage is 31% (renderers are exercised through cli/testscript).
### PR #73 feat(app): undo for every transaction write
## Deviations / notes for G10
- ADR 0016 and the spec (F `fui undo`, T journal) must be amended: scope now includes categorize, tag/untag, rules apply, recurring/loan confirm.
- `rules.Service` gained `WithClock`; `cmd/fui/main.go` wires it.
- No migration.
## Known gaps
- Automatic confirmations made by the importer (loan installments via `lending.ConfirmWithin`, recurrence auto-match in `importing/recur.go`) are not journaled: they are undone with `import undo`.
- The interest category created by a loan confirm stays after undo.
- `tx categorize` has no CLI command, so that kind is covered by app tests, not testscript.
- The do/undo rapid property covers tx add/edit/rm/transfer/split and categorize/tag/untag; recurring and loan confirm use table tests.
### PR #74 feat(app): investments, crypto wallet and asset management gaps
## Deviations / known gaps
- `invest` ops: account-kind rule enforced in plans, not in importing nor in ApplyOp.
- `kpi networth` asset_lines amounts are in each asset's own currency (not base-converted); totals unchanged.
- `main.go` passes the enabled modules to `app.Config` (wallet gate).
- Security price scale is immutable once created.
### PR #75 feat(report): add insight fx, known recurrences and per-account sims
## Deviations
- Warning severity of the CAMT batch record is carried as `raw.severity="warning"` (ErrorRecord has no severity field).
- TUI event parsing duplicates the CLI one (tui cannot import cli); a shared parser in `app/simulating` is a possible follow-up.
- `cmd/fui/tuiv03.go` (not owned) got PlanRemove/ApplyRemove pass-throughs.
- Per-account CSV is long-form (ACCOUNT,MONTH END,BASE,SCENARIO,DELTA).
## Known gaps
None.
