# G04 — Transactions, recurrences, reconciliation, CLI/TUI parity

Branch `feat/g04-tx-recur` · worktree `../fui-wt/g04` · TL: Claude Sonnet medium · deps: none (G03 adds
`ConfirmPending`; if G03 is not merged yet, the TUI confirm calls the recurring confirm and the loan case is wired
when G03 lands — tell the Architect).
Read first: `docs/PLAN-gaps.md` (§2 P1, P10, P15; §5; §6), `docs/tasks/_PROTOCOL.md` (v0.5 addendum),
F: "Requisiti funzionali — CLI", "Requisiti funzionali — TUI", "Modello di dominio" (Ricorrenza), UC-08,
"Riconciliazione", "Decisioni" (Ricorrenze); T: "Architettura", "TUI".

## Owned paths

`internal/app/{transaction.go,tag.go,reconcile.go}` and tests, `internal/app/recurring/`, `internal/app/startup*.go`
(new), `internal/domain/reconcile*.go`, store migration for the tick flag + queries, `cmd/fui/` (startup hook),
`internal/cli/{tx*,recurring*,reconcile*,loan_event*,rule_testcmd*,completion*}.go`, `internal/cli/render/`
matching files, `internal/tui/root.go` (message routing only), `internal/tui/screens/{transactions,settings,accounts}/`.

## Gaps

| Id | Gap | Evidence | Expected |
| --- | --- | --- | --- |
| a2-26 | Recurrences not registered automatically; TUI cannot run/skip/confirm | only callers `cli/recurring_run.go:40`, `loan_run.go:41` | P1: app `Catchup(ctx, today)` (recurrences + loan installments of active modules, idempotent), called by the composition root before command execution (exclusions in P1) and at TUI start; TUI: pending rows in Transactions get `c` confirm (dispatches to recurring or loan confirm) and `S` skip; Settings › Recurrences gets "run until" |
| a2-24 | No payee/tags on add, no payee on edit, no autocomplete | `app/transaction.go:26-35`, `cli/tx_common.go:26-33`, `tx_edit.go:24-31`, `tui/screens/transactions/form.go:42` | `tx add/edit --payee NAME --tag T` (repeatable; unknown payee/tag created with a notice); TUI form fields with autocomplete for account, category, payee and tag (F TUI) |
| a2-27 | No `recurring edit/rm` in CLI | `cli/recurring.go` | `recurring edit NAME [flags]`, `recurring rm NAME` (confirm, `--yes`), same use cases as TUI Settings |
| a2-29 | Reconciliation is all-or-nothing | `app/reconcile.go:33-65` | P10: `reconcile ACCOUNT --date D --balance B [--tick ID…] [--untick ID…]`; shows difference and unticked list; closes at zero difference; TUI Accounts reconcile view lists candidates, `space` ticks, `enter` closes when zero; migration adds the tick flag |
| a2-5 | Async results dropped after a section/tab switch | `tui/screens/settings/model.go:229`, `sections.go:98-104`, `tui/root.go:354-357` | Messages carry their target (section/screen id) and are routed to it even when inactive; test: switch section before the result arrives |
| a2-31 | Spec CLI examples fail | `cli/loan_event.go:23-24`, `cli/rule_testcmd.go:69` | P15: `loan event NAME --rate X --from DATE` (implies rate change), `rule test "TEXT"` without `--account/--amount` (G01 does `budget set`, G06 does `forecast --horizon`) |
| a2-32 | Dynamic completion incomplete | `cli/completion_dynamic.go:39-142` | Names of budgets, recurrences, loans, payees, rules, saved reports (`report run`), securities, assets, reconcile account; silent when archive missing/locked; disabled modules yield nothing |

## Suggested MTs

1. app: `Catchup` + composition hook (CLI pre-run and TUI init) with a fixed-clock test; e2e: second run creates
   nothing.
2. app + cli: payee/tags on add/edit; `recurring edit/rm`; spec-example flags; testscripts.
3. store + domain + app: reconcile ticks (migration, rapid: difference over ticked set independent of tick order).
4. cli: reconcile flags/render; completion extension (tests run `__complete` on a temp archive).
5. tui: async routing fix (root + settings); pending confirm/skip in Transactions; Settings recurrences run.
6. tui: form autocomplete (payee, tag) + reconcile tick view in Accounts; goldens.

## DoD

PLAN §6, plus: fixed-clock test proving startup catch-up is idempotent and skipped for the excluded commands;
teatest for tick → close, pending confirm, autocomplete selection.
