# V26 — TUI: Budget screen
Branch `feat/v-tui-budget`. Owned: `internal/tui/screens/budget/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding). TUI layer: Bubble Tea v1 screen in its own package under `internal/tui/screens/`, following `transactions`/`accounts` (Model, KeyMap with `bubbles/key`, `?` help, use-case ports interface in `ports.go`, theme tokens only — no literal colors), teatest snapshot tests + model unit tests, coverage ≥ 80%. **Do not edit the TUI router/registration**: expose `New(...)`; V32 registers screens per module.

Module `budget`. Spec funzionali.md TUI table row 4: per cap/envelope amount, spent, planned, remaining with carry, progress bar (with text label), "Da assegnare" in envelope mode; actions: set amount, prev/next month, copy previous month, assign/move between envelopes.
