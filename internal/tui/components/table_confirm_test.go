package components_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func TestTableNavigationAndRender(t *testing.T) {
	tbl := components.NewTable([]components.Column{{Title: "A", Width: 3}}, theme.Default())
	tbl.SetColumns([]components.Column{{Title: "Name", Width: 6, Align: lipgloss.Right}, {Title: "N", Width: 2}})
	tbl.SetWidth(15) // exactly the columns plus borders, so fitWidth leaves "Name" at 6 and truncates
	tbl.SetHeight(5) // two visible rows plus the three lines of table chrome
	tbl.SetShowSelect(true)
	if got := tbl.View(); !strings.Contains(got, "(no data)") {
		t.Fatalf("empty view = %q", got)
	}
	if tbl.SelectedRow() != nil {
		t.Fatal("SelectedRow on empty table must be nil")
	}
	tbl.SetCursor(3)
	rows := []components.Row{
		{ID: "1", Cells: []string{"alpha-long-name"}},
		{ID: "2", Cells: []string{"b", "x"}},
		{ID: "3", Cells: []string{"c", "y"}},
		{ID: "4", Cells: []string{"d", "z"}},
	}
	tbl.SetRows(rows)
	if len(tbl.Rows()) != 4 {
		t.Fatal("Rows length")
	}
	tbl.ToggleSelect()
	if got := tbl.View(); !strings.Contains(got, "[x]") || !strings.Contains(got, "…") {
		t.Fatalf("view missing selection mark or truncation: %q", got)
	}
	tbl.PageDown()
	if tbl.Cursor() != 2 {
		t.Fatalf("PageDown cursor = %d, want 2", tbl.Cursor())
	}
	if r := tbl.SelectedRow(); r == nil || r.ID != "3" {
		t.Fatalf("SelectedRow = %v", r)
	}
	tbl.MoveUp(1)
	tbl.PageUp()
	if tbl.Cursor() != 0 {
		t.Fatalf("cursor after up = %d, want 0", tbl.Cursor())
	}
	tbl.SetCursor(-5)
	if tbl.Cursor() != 0 {
		t.Fatal("SetCursor must clamp low")
	}
	tbl.SetCursor(99)
	if tbl.Cursor() != 3 {
		t.Fatal("SetCursor must clamp high")
	}
}

func TestConfirmDialogKeys(t *testing.T) {
	d := components.NewConfirmDialog(theme.Default())
	if c, x := d.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}); c || x {
		t.Fatal("hidden dialog must ignore keys")
	}
	tests := []struct {
		key           tea.KeyMsg
		confirmed, no bool
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}, true, false},
		{tea.KeyMsg{Type: tea.KeyEnter}, true, false},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, false, true},
		{tea.KeyMsg{Type: tea.KeyEsc}, false, true},
	}
	for _, tt := range tests {
		d.Show("Delete", "Sure?", "1 transaction")
		if !strings.Contains(d.View(), "1 transaction") {
			t.Fatal("details missing from view")
		}
		c, x := d.HandleKey(tt.key)
		if c != tt.confirmed || x != tt.no || d.Visible() {
			t.Fatalf("key %q: confirmed=%v cancelled=%v visible=%v", tt.key, c, x, d.Visible())
		}
	}
	d.Show("T", "P")
	if c, x := d.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")}); c || x || !d.Visible() {
		t.Fatal("unrelated key must keep dialog open")
	}
}

// TestTableFitsWidth checks that the widest column absorbs the difference between the columns'
// widths and the table width, so the bordered table spans the screen exactly, but never shrinks
// below a readable minimum.
func TestTableFitsWidth(t *testing.T) {
	cols := []components.Column{{Title: "Date", Width: 10}, {Title: "Description", Width: 24}, {Title: "Amount", Width: 14}}
	for _, width := range []int{80, 60, 40} {
		tbl := components.NewTable(cols, theme.Default())
		tbl.SetRows([]components.Row{{ID: "1", Cells: []string{"2026-01-01", "Rent", "-850.00"}}})
		tbl.SetWidth(width)
		got := lipgloss.Width(tbl.View())
		want := max(width, 10+8+14+4) // the description column stops at 8 cells
		if got != want {
			t.Errorf("width %d: table is %d wide, want %d", width, got, want)
		}
	}
}
