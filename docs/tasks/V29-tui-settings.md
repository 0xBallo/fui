# V29 — TUI: Settings shell, modules, theme picker
Branch `feat/v-tui-settings`. Owned: `internal/tui/screens/settings/` (shell, modules, theme).

Protocol: `docs/tasks/_PROTOCOL.md` (binding). TUI layer: Bubble Tea v1 screen in its own package under `internal/tui/screens/`, following `transactions`/`accounts` (Model, KeyMap with `bubbles/key`, `?` help, use-case ports interface in `ports.go`, theme tokens only — no literal colors), teatest snapshot tests + model unit tests, coverage ≥ 80%. **Do not edit the TUI router/registration**: expose `New(...)`; V32 registers screens per module.

Module core. Settings screen shell with sections; Modules section (list, enable/disable via the app module use cases with dependency warning/confirmation; emits a message so the root rebuilds router/dashboard — V32 handles the rebuild); Appearance section: theme list with live preview, saves `theme` config key.
