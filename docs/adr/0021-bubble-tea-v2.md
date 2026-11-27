# ADR 0021: Migrate the TUI to Bubble Tea v2

* Status: Accepted
* Date: 2026-11-26
* Supersedes: [ADR 0019](0019-tui-libraries.md) (the choice to stay on v1; its library list carries over)

## Context and Problem Statement

ADR 0019 kept the TUI on Bubble Tea v1 and left the v2 migration as a separate project. The v2 lines of every Charm
library FUI uses are now released under `charm.land/*/v2` (Bubble Tea, Bubbles, Lip Gloss, Huh, Glamour), as are
`ntcharts/v2` and `x/exp/teatest/v2`. Only `ethanefung/bubble-datepicker` has no v2 line. Staying on v1 means new
libraries and fixes land only on v2.

## Considered Options

* Stay on v1 until a feature requires v2.
* Migrate the whole ecosystem now, replacing the date picker with an in-house component.

## Decision Outcome

Migrate now. Dependencies move to `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`,
`charm.land/huh/v2`, `charm.land/glamour/v2`, `github.com/NimbleMarkets/ntcharts/v2` and
`github.com/charmbracelet/x/exp/teatest/v2`. `bubble-datepicker` is removed and replaced by
`components.DatePicker`, a small month grid built on the theme tokens (the drop rule of ADR 0019).

How FUI uses v2:

* **Keys** — screens match `tea.KeyPressMsg`; the space key is the binding `"space"`.
* **View** — only `Root.View` returns `tea.View`, declaring the alternate screen and cell-motion mouse mode; screens and
  components keep returning strings.
* **Background** — `Root.Init` sends `tea.RequestBackgroundColor`. Until the reply arrives the theme is resolved from
  `COLORFGBG` (else dark); `tea.BackgroundColorMsg` re-resolves it and rebuilds the screens only if the theme changes.
  No second OSC 11 query runs while the program owns stdin.
* **Colors** — theme tokens are `image/color.Color`. Lip Gloss v2 renders full color and Bubble Tea downsamples at
  output to the terminal profile, honoring `NO_COLOR`; tests pin a profile with `tea.WithColorProfile` or a
  `colorprofile.Writer` instead of a global renderer.

### Consequences

* **Positive:** one framework major across the TUI and its libraries; no global color-profile state, so tests no longer
  depend on run order.
* **Negative:** a one-off diff touching most TUI files and every raw-ANSI golden file; the date picker is now FUI code
  to maintain.
