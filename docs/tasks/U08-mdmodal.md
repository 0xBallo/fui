# U08 — Component: markdown modal (glamour)
Branch `feat/u08-mdmodal`. Owned: `internal/tui/components/mdmodal.go`, `mdmodal_test.go`, goldens, `go.mod`, `go.sum`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. New dependency allowed by ADR 0019: `github.com/charmbracelet/glamour` at the latest **v0.x** that uses lipgloss v1 (check with `go list -m -versions` and `go mod graph`; it must not bump bubbletea/lipgloss to v2).

Read only: `internal/tui/components/modal.go`, `go doc github.com/charmbracelet/bubbles/viewport`, `go doc github.com/charmbracelet/glamour`.

Build:
```go
// MarkdownModal shows a scrollable markdown document framed by Modal.
type MarkdownModal struct{ /* viewport, rendered text, open */ }
func NewMarkdownModal(th theme.Theme, markdown string) MarkdownModal
func (m *MarkdownModal) Open(); func (m *MarkdownModal) Close(); func (m MarkdownModal) IsOpen() bool
func (m *MarkdownModal) SetSize(w, h int) // re-renders with glamour word wrap = inner width
func (m MarkdownModal) Update(msg tea.Msg) (MarkdownModal, tea.Cmd) // up/down/pgup/pgdown/home/end scroll; esc and "i" close
func (m MarkdownModal) View() string
```
Style: `glamour.WithStandardStyle` "dark" or "light" chosen from the theme (add nothing to theme; decide from `th.Name`: latte = light), "notty" for the no-color theme. No literal colors. Render errors fall back to the raw markdown.
Gates: `go test ./cmd/fui/...` (the net/http dependency-graph test) must pass; `go mod tidy`; `govulncheck` if available. If glamour pulls `net/http`, stop: `blocked: glamour imports net/http`.
Tests: renders a heading and a list, scroll moves the view, esc closes, no-color golden.
