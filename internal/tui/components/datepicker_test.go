package components

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/tui/theme"
)

func press(d DatePicker, keys ...tea.KeyMsg) DatePicker {
	for _, k := range keys {
		d, _ = d.Update(k)
	}
	return d
}

func TestDatePicker(t *testing.T) {
	loc := time.FixedZone("UTC+14", 14*3600)
	start := time.Date(2026, 2, 28, 23, 30, 0, 0, loc)
	right := tea.KeyMsg{Type: tea.KeyRight}
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	tests := []struct {
		name   string
		keys   []tea.KeyMsg
		want   string
		wantOK bool
	}{
		{"enter keeps date", []tea.KeyMsg{enter}, "2026-02-28", true},
		{"next day crosses month", []tea.KeyMsg{right, enter}, "2026-03-01", true},
		{"pgdown after next day", []tea.KeyMsg{right, {Type: tea.KeyPgDown}, enter}, "2026-04-01", true},
		{"pgup", []tea.KeyMsg{{Type: tea.KeyPgUp}, enter}, "2026-01-28", true},
		{"esc cancels", []tea.KeyMsg{right, {Type: tea.KeyEsc}, enter}, "", false},
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
