package components

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/tui/theme"
)

// DatePicker is a themed month-grid date chooser. Arrows (or h/j/k/l) move by day/week, pgup/pgdown
// by month, enter picks and esc cancels. It never reads the clock: the caller passes the initial
// date.
type DatePicker struct {
	th     theme.Theme
	day    time.Time // always midnight UTC, so day arithmetic ignores time zones and DST
	open   bool
	picked string
}

// NewDatePicker returns a closed picker styled from th.
func NewDatePicker(th theme.Theme) DatePicker { return DatePicker{th: th} }

// Open shows the picker positioned on the date part of initial.
func (d *DatePicker) Open(initial time.Time) {
	y, mo, day := initial.Date()
	d.day = time.Date(y, mo, day, 0, 0, 0, 0, time.UTC)
	d.open = true
	d.picked = ""
}

// IsOpen reports whether the picker is showing.
func (d DatePicker) IsOpen() bool { return d.open }

// Update handles keys while open and ignores everything otherwise.
func (d DatePicker) Update(msg tea.Msg) (DatePicker, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !d.open || !ok {
		return d, nil
	}
	switch k.String() {
	case "enter":
		d.picked = d.day.Format("2006-01-02")
		d.open = false
	case "esc":
		d.open = false
	case "left", "h":
		d.day = d.day.AddDate(0, 0, -1)
	case "right", "l":
		d.day = d.day.AddDate(0, 0, 1)
	case "up", "k":
		d.day = d.day.AddDate(0, 0, -7)
	case "down", "j":
		d.day = d.day.AddDate(0, 0, 7)
	case "pgup":
		d.day = addMonthsClamped(d.day, -1)
	case "pgdown":
		d.day = addMonthsClamped(d.day, 1)
	}
	return d, nil
}

// addMonthsClamped moves t by n months, clamping the day to the target month's length (Jan 31 +1
// month is Feb 28, not Mar 3).
func addMonthsClamped(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1)
}

// View renders the month grid, or "" when closed.
func (d DatePicker) View() string {
	if !d.open {
		return ""
	}
	const gridWidth = 7*3 - 1
	title := d.day.Format("January 2006")
	pad := max(0, (gridWidth-len(title))/2)
	var b strings.Builder
	b.WriteString(d.th.Header.Render(strings.Repeat(" ", pad)+title) + "\n")
	b.WriteString(d.th.Muted.Render("Su Mo Tu We Th Fr Sa") + "\n")

	first := time.Date(d.day.Year(), d.day.Month(), 1, 0, 0, 0, 0, time.UTC)
	days := first.AddDate(0, 1, -1).Day()
	col := int(first.Weekday())
	b.WriteString(strings.Repeat("   ", col))
	for day := 1; day <= days; day++ {
		cell := fmt.Sprintf("%2d", day)
		if day == d.day.Day() {
			b.WriteString(d.th.Selected.Render(cell))
		} else {
			b.WriteString(d.th.Base.Render(cell))
		}
		col++
		switch {
		case day == days:
		case col == 7:
			b.WriteString("\n")
			col = 0
		default:
			b.WriteString(" ")
		}
	}
	return d.th.Base.Padding(1, 1).Render(b.String())
}

// Picked returns the chosen date as "YYYY-MM-DD" once after enter, then clears it.
func (d *DatePicker) Picked() (string, bool) {
	p := d.picked
	d.picked = ""
	return p, p != ""
}
