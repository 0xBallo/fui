# G03 — Loans through import and export

Branch `feat/g03-loan-import` · worktree `../fui-wt/g03` · TL: Claude Sonnet medium · deps: **G02 merged**.
Read first: `docs/PLAN-gaps.md` (§5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum), F: "Mutui e prestiti",
"Formato FUI" (`schedule`, `installment`), "Import, export e riconciliazione"; T: "Invarianti" (amortization).

## Owned paths

`internal/app/importing/{loan*,build.go,rows.go,recur.go,export.go}` (coordinate nothing: G02 is merged),
`internal/app/lending/`, `internal/testutil/memstore/loan*`, `internal/cli/loan*.go`, store loan queries/repo
(migration only if needed), `pkg/fuiproto/` (only if needed).

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a2-8 | `fui import` drops `schedule`/`installment` | `importing/build.go:127-140`; `cli/loan_common.go:34-37` | Import creates/updates loans: each `schedule` record with a new `valid_from` becomes a new schedule version (rate, type, valid_from honoured); `installment` rows become the version's installments (bank figures override computed); dedup on re-import; gated by `loans` (G02 dialog) |
| a2-9 | FUI export lacks loans | `importing/export.go:312-347` | Export writes `schedule` + `installment` records for every loan version; round trip export → import on an empty archive reproduces loans |
| a2-10 | Import does not match pending installments; TUI edit skips the split | `importing/recur.go:55-57`, `lending/installments.go:158-161`, `tui/screens/transactions/form.go:155` | Import matches a booked debit to the pending installment (amount tolerance and date window as recurrences) and confirms it through the lending confirm path (principal transfer + interest); TUI confirm of a pending installment calls the lending confirm (G04 adds the TUI key; here expose the app entry `ConfirmPending(ctx, txID)` that dispatches recurrence vs installment) |
| — | `loan add --import` keeps working on top of the new import path (no duplicate code) | `cli/loan_common.go` | `loan add --import FILE` reuses the importing loan builder |

## Suggested MTs

1. app/lending: version upsert from imported schedule records + installment overrides; rapid: principal sum =
   capital and final residual 0 after an imported renegotiation.
2. app/importing: build/apply of schedule/installment, dedup, plan preview lines.
3. app/importing: export records + round-trip test; `ConfirmPending` dispatcher; installment matching on import.
4. cli: `loan add --import` refactor, testscript `g03_loan_import.txtar` (import bank plan, renegotiation version,
   import matching debit → split, export round trip).

## DoD

PLAN §6, plus the round-trip testscript and the rapid property above.
