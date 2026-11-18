# U11 — Root: bottom help bar
Branch `feat/u11-root-help`. Owned: `internal/tui/root.go`, `internal/tui/keymap.go`, `internal/tui/root_test.go`, `internal/tui/teatest_test.go`, `internal/tui/testdata/`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `internal/tui/root.go`, `internal/tui/components/helpbar.go` (U05).

Do:
1. Remove the hint line under the tab bar (`viewHints`, `isNavigation`) and the help modal (`viewHelp`, `showHelp` as modal).
2. Layout: tabs, blank, body, status line (error/status text, empty when none), help bar. Collapsed help = 1 line; `?` toggles `HelpBar` expanded **in place** (the body shrinks by the expanded height; `esc` or `?` collapses; any key while expanded only collapses, as today). Update `reservedRows` and `resize()` so screens get the exact remaining height in both states (send a resize to screens on toggle).
3. Global group = `km.Bindings()`, screen group = `ActiveKeyMap()` titled with the screen `Title()`.
4. Status bar default text `? help · q quit` goes away (the help bar replaces it).
5. Mouse: a left click anywhere still collapses the expanded help.
Tests: 80×24 snapshot has the help as the last row and 1 line high; long keymap is cut with `…`; `?` expands with both groups and the screen gets a smaller height; regenerate root goldens (`-update`) and review the diff.
