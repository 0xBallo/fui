# V32 — Integration v0.2
Branch `feat/v-integration`. Owned: `cmd/fui/`, `internal/cli/root.go` + `help.json` goldens, TUI router/registration, `internal/skill/`, `testdata/e2e/`, `.github/`, `docs/adr/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding).

Register all v0.2 CLI commands (module annotations) and TUI screens per module (router rebuild on module change, fixed numeric keys); `help --json` golden per preset (expense, full) now differs; testscript e2e per use case (budget, recurring, loans, rules, reconcile, tags/splits, reports, forecast) and the module matrix (each optional module off once); `module_disabled` exercised; skill updated; coverage gate green; CI green.
