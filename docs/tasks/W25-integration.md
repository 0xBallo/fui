# W25 — Integration

Branch `feat/w-integration`. Owned: `cmd/fui/`, `internal/cli/root.go`, `internal/tui/register.go`, `internal/connector/registry.go`, `internal/skill/`, `testdata/`, `docs/adr/`, `.github/` if needed. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Compose all services; register OFX/CAMT connectors and new commands/screens only for active modules; testscript e2e: OFX and CAMT import (+undo), investments flow (buy/sell/dividend/position), assets + net worth, simulate (no tx written), insights dismiss/accept, `fui undo`; module matrix (`expense`, `full`, each v0.3 module off) with per-preset `help --json` golden; skill updated; ADRs 0014+ for decisions in the plan; coverage gate (total ≥ 80%, domain ≥ 90%) and CI green.
