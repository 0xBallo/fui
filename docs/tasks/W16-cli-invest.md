# W16 — CLI: security, invest, position

Branch `feat/w-cli-invest`. Owned: `internal/cli/{security,invest,position}*.go`, `internal/cli/render/` new files, runway `--include-investments` in `kpi_runway.go`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Commands `fui security add/list/price`, `fui invest buy/sell/dividend`, `fui position list` with `--json/--csv`, `--dry-run`, module annotation (`investments`; crypto kinds need `crypto`), examples on each, help --json golden. Constructors `NewSecurityCmd(deps)` etc.; do not edit root.go.
Accept: command tests + goldens; e2e is W25.
