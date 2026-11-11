package components_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func TestTable(t *testing.T) {
	cols := []components.Column{
		{Title: "ID", Width: 4, Align: lipgloss.Left},
		{Title: "Name", Width: 10, Align: lipgloss.Left},
	}
	tbl := components.NewTable(cols, theme.Default())
	tbl.SetHeight(3)

	// Empty view
	if view := tbl.View(); !strings.Contains(view, "(no data)") {
		t.Fatalf("expected '(no data)' in empty view, got: %s", view)
	}

	rows := []components.Row{
		{ID: "1", Cells: []string{"1", "Alpha"}},
		{ID: "2", Cells: []string{"2", "Beta"}},
		{ID: "3", Cells: []string{"3", "Gamma"}},
		{ID: "4", Cells: []string{"4", "Delta"}},
	}
	tbl.SetRows(rows)

	if diff := cmp.Diff(0, tbl.Cursor()); diff != "" {
		t.Errorf("cursor mismatch (-want +got):\n%s", diff)
	}
	if row := tbl.SelectedRow(); row == nil || row.ID != "1" {
		t.Fatalf("expected selected row '1', got %+v", row)
	}

	tbl.MoveDown(1)
	if tbl.Cursor() != 1 {
		t.Errorf("expected cursor 1, got %d", tbl.Cursor())
	}

	tbl.MoveBottom()
	if tbl.Cursor() != 3 {
		t.Errorf("expected cursor 3, got %d", tbl.Cursor())
	}
	if !tbl.NearBottom(1) {
		t.Errorf("expected NearBottom to be true")
	}

	tbl.MoveTop()
	if tbl.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", tbl.Cursor())
	}
	if !tbl.NearTop(1) {
		t.Errorf("expected NearTop to be true")
	}

	// Multi-selection
	tbl.SetShowSelect(true)
	tbl.ToggleSelect()
	if diff := cmp.Diff([]string{"1"}, tbl.SelectedIDs()); diff != "" {
		t.Errorf("selected IDs mismatch (-want +got):\n%s", diff)
	}
	tbl.ToggleSelect()
	if len(tbl.SelectedIDs()) != 0 {
		t.Errorf("expected empty selection after toggle")
	}

	tbl.ToggleSelect()
	tbl.ClearSelection()
	if len(tbl.SelectedIDs()) != 0 {
		t.Errorf("expected empty selection after ClearSelection")
	}
}

func TestBarChart(t *testing.T) {
	chart := components.NewBarChart(theme.Default())
	if view := chart.View(); !strings.Contains(view, "(no data)") {
		t.Errorf("expected '(no data)' for empty chart, got: %s", view)
	}

	items := []components.BarItem{
		{Label: "Groceries", Value: 200, DisplayValue: "200.00 EUR"},
		{Label: "Rent", Value: 1000, DisplayValue: "1000.00 EUR"},
	}
	chart.SetItems(items)
	chart.SetWidth(50)

	view := chart.View()
	if !strings.Contains(view, "Groceries") || !strings.Contains(view, "Rent") {
		t.Errorf("expected view to contain items, got: %s", view)
	}
}

func TestConfirmDialog(t *testing.T) {
	dlg := components.NewConfirmDialog(theme.Default())
	if dlg.Visible() {
		t.Errorf("expected dialog to start hidden")
	}

	dlg.Show("Confirm", "Are you sure?", "Extra detail")
	if !dlg.Visible() {
		t.Errorf("expected dialog to be visible")
	}
	if view := dlg.View(); !strings.Contains(view, "Are you sure?") {
		t.Errorf("expected prompt in view, got: %s", view)
	}

	dlg.Hide()
	if dlg.Visible() {
		t.Errorf("expected dialog to be hidden after Hide")
	}
}
