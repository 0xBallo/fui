# U26 — Reports: bar charts
Branch `feat/u26-report-bars`. Owned: `internal/tui/screens/report/{charts_bar.go,charts_bar_test.go}`, one dispatch line per kind in `charts.go`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `report/charts.go` (U25), `report/compare.go`, `components/barchart.go` (`BarItem.Style`).

Do: horizontal bars (`components.BarChart`) for **Budget vs actual** (two bars per category: budget `th.Muted`, actual tier-colored like U22 or `Negative` if over), **Top payees**, **Interest paid**, and **Compare** (period A `Series1` vs B `Series2` per category). Replace any older ad-hoc bars for those kinds.
Tests: pure builders per kind, sign handling, goldens for one kind.
