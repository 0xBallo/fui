package components

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/tui/theme"
)

func press(d DatePicker, keys ...tea.KeyPressMsg) DatePicker {
	for _, k := range keys {
		d, _ = d.Update(k)
	}
	return d
}

func TestDatePicker(t *testing.T) {
	loc := time.FixedZone("UTC+14", 14*3600)
	start := time.Date(2026, 2, 28, 23, 30, 0, 0, loc)
	right := tea.KeyPressMsg{Code: tea.KeyRight}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	tests := []struct {
		name   string
		keys   []tea.KeyPressMsg
		want   string
		wantOK bool
	}{
		{"enter keeps date", []tea.KeyPressMsg{enter}, "2026-02-28", true},
		{"next day crosses month", []tea.KeyPressMsg{right, enter}, "2026-03-01", true},
		{"pgdown after next day", []tea.KeyPressMsg{right, {Code: tea.KeyPgDown}, enter}, "2026-04-01", true},
		{"pgup", []tea.KeyPressMsg{{Code: tea.KeyPgUp}, enter}, "2026-01-28", true},
		{"esc cancels", []tea.KeyPressMsg{right, {Code: tea.KeyEsc}, enter}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDatePicker(theme.Default())
			d.Open(start)
			if !d.IsOpen() || d.View() == "" {
				t.Fatal("expected open picker with a view")
			}
			d = press(d, tc.keys...)
			got, ok := d.Picked()
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("Picked() = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
			if _, ok := d.Picked(); ok {
				t.Fatal("Picked must clear after first read")
			}
			if d.IsOpen() || d.View() != "" {
				t.Fatal("picker must be closed")
			}
		})
	}
}
