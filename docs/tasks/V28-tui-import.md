# V28 — TUI: Import screen
Branch `feat/v-tui-import`. Owned: `internal/tui/screens/importer/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). TUI layer: Bubble Tea v1 screen in its own package under `internal/tui/screens/`, following `transactions`/`accounts` (Model, KeyMap with `bubbles/key`, `?` help, use-case ports interface in `ports.go`, theme tokens only — no literal colors), teatest snapshot tests + model unit tests, coverage ≥ 80%. **Do not edit the TUI router/registration**: expose `New(...)`; V32 registers screens per module.

Module core. Choose connector/profile and file, preview rows (new/duplicate/doubtful highlighted with symbols, not only color), column mapping for CSV, include/exclude rows, unmapped accounts mapping, rule effects (V14), confirm → apply via the importing service.
