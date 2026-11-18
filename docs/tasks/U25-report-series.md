# U25 — Reports: series charts
Branch `feat/u25-report-series`. Owned: `internal/tui/screens/report/{charts.go,charts_test.go,view.go}` and goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `report/{view,rows,report,kinds}.go`, `components/linechart.go` (U09), result types in `internal/app/reporting/{networth,forecast,analytics}.go` (only the ones you chart).

Do: `charts.go` with `func (m *Model) chartView(width, height int) string` dispatching by kind; `view.go` calls it instead of the current chart line (keep bar charts that exist today until U26). Line charts (`components.LineChart`, Y formatted with `domain.FormatAmount`): **Net worth** (total + liquidity series), **Balance over time**, **Forecast** (total projected balance), **Remaining debt**. Chart above the table, height `min(12, h/2)`; hidden when h < 18.
Tests: each kind maps to the expected series (assert on `[]components.Series` from a pure builder, not ANSI), small height hides the chart, goldens for one kind.
