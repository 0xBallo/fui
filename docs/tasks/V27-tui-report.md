# V27 — TUI: Report screen
Branch `feat/v-tui-report`. Owned: `internal/tui/screens/report/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). TUI layer: Bubble Tea v1 screen in its own package under `internal/tui/screens/`, following `transactions`/`accounts` (Model, KeyMap with `bubbles/key`, `?` help, use-case ports interface in `ports.go`, theme tokens only — no literal colors), teatest snapshot tests + model unit tests, coverage ≥ 80%. **Do not edit the TUI router/registration**: expose `New(...)`; V32 registers screens per module.

Module `analytics`. Predefined + saved reports with text bar charts (reuse `components.BarChart`) and tables, forecast view; change report/period, export (calls the app export).
