# V24 — CLI: forecast, reports, net worth
Branch `feat/v-cli-report`. Owned: `internal/cli/{forecast,report*,kpi_networth*}.go`, render files.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). CLI layer rules as in `V20-cli-split.md` (thin commands, module annotation, no `root.go`/`help.json` edits — V32 registers; test constructors directly).

Module `analytics`. `fui forecast [--horizon N]`, `fui report <kind> [--period ...] [--chart]` for the new reports (text bar chart reuse), `fui report save/list/run/rm`, `fui kpi networth` includes assets/loan debt when modules are on.
