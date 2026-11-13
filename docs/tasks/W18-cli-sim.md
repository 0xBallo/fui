# W18 — CLI: simulate

Branch `feat/w-cli-sim`. Owned: `internal/cli/simulate*.go`, `internal/cli/render/simulate.go`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

`fui simulate investment|pac|cashflow|save|list|compare` (example `fui simulate pac --monthly 200 --years 20 --rate 4,5`), three-rate output side by side, `--json`, `--chart`, module `simulations`; never writes transactions.
