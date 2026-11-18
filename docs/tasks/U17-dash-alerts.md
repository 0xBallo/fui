# U17 — Dashboard: alerts
Branch `feat/u17-dash-alerts`. Owned: `internal/tui/screens/dashboard/{alerts,insight}*.go` and their tests/goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Wiring note: `cmd/fui` passes `tuiUseCases{*app.Service, ...}`, so a method of `*app.Service` added to a screen `UseCases` interface needs no wiring; anything else is an **optional port** with a setter, wired by U27. Do not edit `model.go` beyond removing the insight panel render line if U13 left it (one line).

Read only: `dashboard/section.go`, `dashboard/alerts.go` (stub), `dashboard/insight.go`, `budgeting.Status` (`internal/app/budgeting/status.go`), `domain.BudgetState`, `app.Recurrence` and `domain.Expand` (`internal/domain/recurrence.go`).

Do: one boxed panel (`th.Border`) titled `Alerts`, entries sorted danger → warning → info, each `icon level text`:
- **danger** `✗` (`th.Error`): budget caps with `BudgetOver`, envelopes with overspend — month of `AsOf`.
- **warning** `!` (`th.Warning`): caps with `BudgetWarning`; anomaly insights.
- **info** `i` (`th.Info`): expense recurrences with an occurrence in [today, end of the current billing month = `MonthPeriod(today, MonthStartDay).To`] (`Rent −800,00 on 2026-11-27`); subscription/trend insights.
Ports (optional, each nil = source skipped): `AlertBudgetUseCases{ Status(ctx, month string) (budgeting.Status, error) }`, `AlertRecurringUseCases{ ListRecurrences(ctx) ([]app.Recurrence, error) }` (U27 adapts the module set), existing `InsightUseCases`. Setters `SetAlertBudget`, `SetAlertRecurring`; `SetInsights` keeps its name.
Absorb the insight panel: `i` focuses the alerts list (j/k move), `x` dismiss and `a` accept act on a focused insight entry exactly as today; then delete the old panel view code. No alerts → muted "all clear".
Tests: ordering, each source mapping, recurrence window edges (today, end of month, outside), focus + dismiss/accept still work (port the existing insight tests), golden with all three levels.
