# U21 — Accounts: currency, sections, totals
Branch `feat/u21-accounts`. Owned: `internal/tui/screens/accounts/` except `pos*`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Wiring note: `cmd/fui` passes `tuiUseCases{*app.Service, ...}`, so a method of `*app.Service` added to a screen `UseCases` interface needs no wiring; anything else is an **optional port** with a setter, wired by U27.

Read only: `accounts/{model,keymap,ports}.go`, `app.NetWorth`/`NetWorthLine` in `internal/app/fx.go`, `domain.AccountLoan`.

Do: add a `Currency` column. Split the list into two tables: **Accounts** (every kind but loan — they make up gross wealth) and **Loans & mortgages** (kind loan); hide the second when empty. `tab` switches focus (help `tab switch section`); open/archive/reconcile act on the focused table's row. Under each table `Total <amount> <BASE>` from `NetWorth` lines (`Converted`), added to `UseCases`; no base currency or missing rate → `Total n/a`.
Tests: split by kind, focus switching, actions target the focused row, totals and n/a, goldens for this package.
