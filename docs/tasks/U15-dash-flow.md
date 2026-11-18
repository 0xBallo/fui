# U15 — Dashboard: flow chart
Branch `feat/u15-dash-flow`. Owned: `internal/tui/screens/dashboard/flow.go`, `flow_test.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Wiring note: `cmd/fui` passes `tuiUseCases{*app.Service, ...}`, so a method of `*app.Service` added to a screen `UseCases` interface needs no wiring; anything else is an **optional port** with a setter, wired by U27. Do not edit `model.go`.

Read only: `dashboard/section.go`, `dashboard/flow.go` (stub), `reporting.CashFlowParams/CashFlowResult` in `internal/app/reporting/cashflow.go`, `components/barchart.go`.

Do: optional port in `flow.go`:
```go
type FlowUseCases interface{ CashFlow(ctx context.Context, p reporting.CashFlowParams) (reporting.CashFlowResult, error) }
func (m *Model) SetFlowUseCases(uc FlowUseCases)
```
Load `CashFlow{Period: p.Period, BaseCurrency: p.Base, MonthStartDay: p.MonthStartDay}`; render a `components.BarChart` with three items in this order: `Income` (TotalInflow, `th.Positive` style), `Expenses` (|TotalOutflow|, `th.Negative`), `Savings` (TotalNet, `th.Base`; negative → bar of the absolute value, sign in the label). Values via `domain.FormatAmount` with the sign. Title `Cash flow`.
Tests: bars order/styles/values, negative savings, disabled port hides the section.
