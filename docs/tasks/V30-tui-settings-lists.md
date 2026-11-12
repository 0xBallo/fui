# V30 — TUI: Settings lists
Branch `feat/v-tui-settings-lists`. Owned: `internal/tui/screens/settings/lists/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). TUI layer: Bubble Tea v1 screen in its own package under `internal/tui/screens/`, following `transactions`/`accounts` (Model, KeyMap with `bubbles/key`, `?` help, use-case ports interface in `ports.go`, theme tokens only — no literal colors), teatest snapshot tests + model unit tests, coverage ≥ 80%. **Do not edit the TUI router/registration**: expose `New(...)`; V32 registers screens per module.

CRUD lists for categories, tags, rules (with test preview), recurrences, import profiles, each a sub-view of the V29 shell (expose constructors; V29 shell integration in a small commit if merged first, else REQ).
