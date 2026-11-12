# T19 — Catppuccin themes with semantic tokens

Branch `feat/tui-themes`. Owned paths: `internal/tui/theme/`, other `internal/tui/**` files only to replace
literal colors/old style names with tokens, `internal/config/` (only `theme`, `theme_light`, `theme_dark`),
`.golangci.yml` (forbidigo/depguard rule: no `lipgloss.Color`/catppuccin outside `internal/tui/theme`),
`go.mod`/`go.sum` (`github.com/catppuccin/go` direct). Protocol: `docs/tasks/_PROTOCOL.md`.

Spec: tecniche.md TUI "Layout e tema", "Temi", "Rilevamento dello sfondo", "Token semantici", "Profondità di
colore e NO_COLOR", Configurazione `theme*` keys, Test "Temi" + the two theme invariants. Bubble Tea is v1.3
(`go doc github.com/charmbracelet/bubbletea` / lipgloss v1.1 — `lipgloss.HasDarkBackground`, termenv OSC 11).

1. Tokens (`Base, Surface, Selection, Text, Muted, Border, Accent, Income, Expense, Warning, Over, Pending,
   Reconciled, Series1..8`) mapped for latte/frappe/macchiato/mocha, plus `terminal` (16 ANSI colors) and
   `high-contrast`. Keep/adapt the existing `Theme` styles so screens build from tokens.
2. `Resolve(cfg, detector) Theme`: `auto` uses `theme_light`/`theme_dark` from a background detector: OSC 11
   with 100 ms timeout → `COLORFGBG` → dark. Detector is an interface so tests simulate light/dark/timeout.
   `NO_COLOR` → no colors; <256-color terminals → `terminal`.
3. Config keys with defaults `auto`/`latte`/`mocha`; unknown value → error listing valid themes.
4. Tests: WCAG contrast ≥ 4.5:1 for text tokens vs `Base` (≥ 7:1 for high-contrast) for every theme; every theme
   defines every token (reflection or table test); teatest snapshots of Dashboard and Transactions for each theme
   and `NO_COLOR` (keep the goldens small); detector tests. Settings screen is v0.2: theme choice is via
   `fui config set theme <v>` only.
