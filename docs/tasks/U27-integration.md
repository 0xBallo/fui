# U27 — Integration: wiring, goldens, e2e
Branch `feat/u27-integration`. Owned: `cmd/fui/`, `internal/tui/root.go` (Config + rebuild only), `internal/tui/*_test.go`, `internal/tui/testdata/`, `CHANGELOG.md`, `docs/PLAN-tui-revamp.md` (status).

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Read every `.task-report.md` `REQ:` line addressed to U27 first.

Do:
1. `tui.Config`: add `Flow`, `AlertBudget`, `AlertRecurring` ports and `Version`; `newDashboard` sets them only when their module is enabled (flow = core, budget = `module.Budget`, recurring = `module.Recurring`).
2. `cmd/fui`: adapters (`CashFlow` → `cli.Reporting.CashFlow.Run`, budget status → `budgeting.Service.Status`, recurrences → `recurring.Service.List(modules)`), pass `version`.
3. Root tests: theme snapshot and v0.3 goldens regenerated and reviewed; a teatest flow: dashboard → `p` cycles → `t` opens Transactions → `?` expands help → `4` Budget → `i` modal → esc; 80×24 renders without overflow on every tab.
4. Sandboxed manual check (`_PROTOCOL.md`): `make demo` archive, `go run ./cmd/fui` with all XDG paths under `./.scratch`; describe what you saw in the report.
5. CHANGELOG "Unreleased" entry; plan status table.
Gates: whole-repo checks of the protocol, coverage ≥ 80% on every touched package.
