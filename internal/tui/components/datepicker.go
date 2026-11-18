package components

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	datepicker "github.com/ethanefung/bubble-datepicker"

	"github.com/0xballo/fui/internal/tui/theme"
)

// DatePicker is a themed month-grid date chooser. Arrows move by day/week, pgup/pgdown by month,
// enter picks and esc cancels. It never reads the clock: the caller passes the initial date.
type DatePicker struct {
	m      datepicker.Model
	th     theme.Theme
	open   bool
	picked string
}

// NewDatePicker returns a closed picker styled from th.
func NewDatePicker(th theme.Theme) DatePicker {
	m := datepicker.New(time.Time{})
	m.Styles = datepicker.Styles{
		Header:       th.Base.Padding(1, 0, 0),
		Date:         th.Base.Padding(0, 1, 1),
		HeaderText:   th.Header,
		Text:         th.Muted,
		SelectedText: th.Selected,
		FocusedText:  th.Selected,
	}
	// The library's quit binding would emit tea.Quit; closing is the caller's esc handling.
	m.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	return DatePicker{m: m, th: th}
}

// Open shows the picker positioned on the date part of initial.
func (d *DatePicker) Open(initial time.Time) {
	y, mo, day := initial.Date()
	d.m.SetTime(time.Date(y, mo, day, 0, 0, 0, 0, time.UTC))
	d.m.SetFocus(datepicker.FocusCalendar)
	d.open = true
	d.picked = ""
}

// IsOpen reports whether the picker is showing.
func (d DatePicker) IsOpen() bool { return d.open }

// Update handles keys while open and ignores everything otherwise.
func (d DatePicker) Update(msg tea.Msg) (DatePicker, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !d.open || !ok {
		return d, nil
	}
	switch k.String() {
	case "enter":
		d.picked = d.m.Time.Format("2006-01-02")
		d.open = false
	case "esc":
		d.open = false
	case "pgup":
		d.m.LastMonth()
	case "pgdown":
		d.m.NextMonth()
	default:
		d.m, _ = d.m.Update(msg)
	}
	return d, nil
}

// View renders the month grid, or "" when closed.
func (d DatePicker) View() string {
	if !d.open {
		return ""
	}
	return d.m.View()
}

// Picked returns the chosen date as "YYYY-MM-DD" once after enter, then clears it.
func (d *DatePicker) Picked() (string, bool) {
	p := d.picked
	d.picked = ""
	return p, p != ""
}
