# U05 — Component: help bar
Branch `feat/u05-helpbar`. Owned: `internal/tui/components/helpbar.go`, `helpbar_test.go`, goldens.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low.

Read only: `internal/tui/root.go` (`viewHints`, `isNavigation`, `viewHelp` — the behavior you replace), `go doc github.com/charmbracelet/bubbles/help Model`.

Build on `bubbles/help` (no new dependency):
```go
type HelpBar struct{ /* help.Model, width, expanded */ }
func NewHelpBar(th theme.Theme) HelpBar
func (h *HelpBar) SetWidth(w int)
func (h *HelpBar) Toggle()            // collapsed <-> expanded
func (h HelpBar) Expanded() bool
// View: collapsed = ONE line, screen bindings first then global, navigation keys skipped (move
// isNavigation here), cut with "…" when too wide (help.Model.ShortHelpView with Width set).
// Expanded = two titled groups "Global" and screenTitle, each "key  desc" lines, laid out in
// side-by-side columns when taller than maxHeight (port the column logic of Root.viewHelp).
func (h HelpBar) View(global, screen []key.Binding, screenTitle string, maxHeight int) string
```
Use `th.HelpKey`/`th.HelpDesc` styles. Disabled bindings and bindings without help are skipped.
Tests: collapsed fits exactly 1 line at width 40/80/200 and ends with `…` when cut; expanded shows both group titles; column wrap at maxHeight 5; goldens.
Out of scope: wiring into Root (U11).
