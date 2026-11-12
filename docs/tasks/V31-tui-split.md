# V31 — TUI: split editor and reconcile
Branch `feat/v-tui-split`. Owned: `internal/tui/screens/{transactions,accounts}/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). TUI layer: Bubble Tea v1 screen in its own package under `internal/tui/screens/`, following `transactions`/`accounts` (Model, KeyMap with `bubbles/key`, `?` help, use-case ports interface in `ports.go`, theme tokens only — no literal colors), teatest snapshot tests + model unit tests, coverage ≥ 80%. **Do not edit the TUI router/registration**: expose `New(...)`; V32 registers screens per module.

Transactions: split editor (rows, live remaining amount, validation), tag editing, bulk categorize. Accounts: reconcile action (statement balance + date, difference, close at zero).
