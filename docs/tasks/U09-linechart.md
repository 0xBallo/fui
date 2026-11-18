# U09 — Component: line chart (ntcharts)
Branch `feat/u09-linechart`. Owned: `internal/tui/components/linechart.go`, `linechart_test.go`, goldens, `go.mod`, `go.sum`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. New dependency allowed by ADR 0019: `github.com/NimbleMarkets/ntcharts` from its Bubble Tea **v1** line (module path without `/v2`; pick the latest tag/commit of branch `main` whose go.mod requires bubbletea v1 and lipgloss v1).

Read only: `internal/tui/components/barchart.go` (component style), `go doc github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart` (or `linechart`).

Build a domain-free wrapper used by Reports:
```go
type Point struct{ Label string; Value int64 } // Label = x tick ("2026-03"), Value = minor units
type Series struct{ Name string; Points []Point; Style lipgloss.Style }
type LineChart struct{ /* ntcharts model, theme, size */ }
func NewLineChart(th theme.Theme) LineChart
func (c *LineChart) SetSize(w, h int)
func (c *LineChart) SetSeries(s ...Series)        // up to 4 series; Y axis labels formatted by caller-provided func
func (c *LineChart) SetYFormatter(f func(int64) string)
func (c LineChart) View() string                  // legend line under the plot; "(no data)" when empty
```
Axis/labels use `th.Muted`, default series styles `th.Tokens.Series1..4`. No literal colors.
Gates: net/http graph test green, `go mod tidy`.
Tests: golden for 1 and 2 series at 60×12, empty series, negative values, deterministic output.
