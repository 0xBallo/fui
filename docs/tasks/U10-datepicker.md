# U10 — Component: date picker
Branch `feat/u10-datepicker`. Owned: `internal/tui/components/datepicker.go`, `datepicker_test.go`, goldens, `go.mod`, `go.sum`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. New dependency allowed by ADR 0019: `github.com/ethanefung/bubble-datepicker` (Bubble Tea v1). Components never import `internal/domain` and never call `time.Now()`: the initial date is passed in.

Read only: `go doc -all github.com/ethanefung/bubble-datepicker`, `internal/tui/components/modal.go`.

Build:
```go
type DatePicker struct{ /* wrapped model */ }
func NewDatePicker(th theme.Theme) DatePicker
func (d *DatePicker) Open(initial time.Time) // date only, UTC midnight
func (d DatePicker) IsOpen() bool
// Update: arrows move day/week, pgup/pgdown month, enter picks, esc cancels.
func (d DatePicker) Update(msg tea.Msg) (DatePicker, tea.Cmd)
func (d DatePicker) View() string
// Picked returns the chosen date as "YYYY-MM-DD" once, after enter, then clears it.
func (d *DatePicker) Picked() (string, bool)
```
Map the library's styles onto theme styles (`Selected`, `Muted`, `Header`). If the library cannot be themed without literal colors or does not build with our bubbletea version, implement a minimal month grid yourself (≤ 150 lines, no dependency) and say so in `DEV:`.
Tests: open on 2026-02-28, move +1 day → 2026-03-01, pgdown, enter returns the string, esc returns nothing; `TZ=Pacific/Kiritimati` must not shift the date.
