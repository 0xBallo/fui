// Package components provides domain-agnostic UI building blocks for the TUI.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/tui/theme"
)

// Column defines a table column header, width and content alignment.
type Column struct {
	Title string
	Width int
	Align lipgloss.Position
}

// Row holds the unique identifier and cell contents for a table row.
type Row struct {
	ID    string
	Cells []string
}

// Table displays a scrollable, virtualized list of rows, rendering only visible items.
type Table struct {
	columns    []Column
	rows       []Row
	cursor     int
	offset     int
	height     int
	width      int
	selected   map[string]bool
	showSelect bool
	theme      theme.Theme
}

// NewTable constructs a table with columns and an active theme.
func NewTable(columns []Column, th theme.Theme) Table {
	return Table{
		columns:  columns,
		selected: make(map[string]bool),
		theme:    th,
		height:   10,
	}
}

// SetColumns replaces the table column definitions.
func (t *Table) SetColumns(columns []Column) {
	t.columns = columns
}

// SetRows updates row data, preserving cursor within the new valid range.
func (t *Table) SetRows(rows []Row) {
	t.rows = rows
	if t.cursor >= len(rows) {
		t.cursor = max(0, len(rows)-1)
	}
	t.adjustOffset()
}

// Rows returns the current table rows.
func (t *Table) Rows() []Row {
	return t.rows
}

// SetHeight sets the maximum number of rows visible in the viewport.
func (t *Table) SetHeight(h int) {
	if h > 0 {
		t.height = h
		t.adjustOffset()
	}
}

// SetWidth sets the available display width for the table.
func (t *Table) SetWidth(w int) {
	t.width = w
}

// SetShowSelect enables or disables the multi-selection checkbox column.
func (t *Table) SetShowSelect(show bool) {
	t.showSelect = show
}

// Cursor returns the index of the currently highlighted row.
func (t *Table) Cursor() int {
	return t.cursor
}

// SetCursor moves the highlight to a specific index and adjusts scroll position.
func (t *Table) SetCursor(idx int) {
	if len(t.rows) == 0 {
		t.cursor = 0
		t.offset = 0
		return
	}
	if idx < 0 {
		idx = 0
	} else if idx >= len(t.rows) {
		idx = len(t.rows) - 1
	}
	t.cursor = idx
	t.adjustOffset()
}

// SelectedRow returns a pointer to the highlighted row, or nil if empty.
func (t *Table) SelectedRow() *Row {
	if len(t.rows) == 0 || t.cursor < 0 || t.cursor >= len(t.rows) {
		return nil
	}
	return &t.rows[t.cursor]
}

// ToggleSelect toggles the checked state of the currently highlighted row.
func (t *Table) ToggleSelect() {
	row := t.SelectedRow()
	if row == nil {
		return
	}
	if t.selected[row.ID] {
		delete(t.selected, row.ID)
	} else {
		t.selected[row.ID] = true
	}
}

// ClearSelection removes all multi-selection marks.
func (t *Table) ClearSelection() {
	t.selected = make(map[string]bool)
}

// SelectedIDs returns all IDs marked in multi-selection mode.
func (t *Table) SelectedIDs() []string {
	ids := make([]string, 0, len(t.selected))
	for id := range t.selected {
		ids = append(ids, id)
	}
	return ids
}

// MoveUp shifts the cursor upwards by n rows.
func (t *Table) MoveUp(n int) {
	t.SetCursor(t.cursor - n)
}

// MoveDown shifts the cursor downwards by n rows.
func (t *Table) MoveDown(n int) {
	t.SetCursor(t.cursor + n)
}

// PageUp moves the cursor up by the visible viewport height.
func (t *Table) PageUp() {
	t.MoveUp(max(1, t.height))
}

// PageDown moves the cursor down by the visible viewport height.
func (t *Table) PageDown() {
	t.MoveDown(max(1, t.height))
}

// MoveTop jumps the cursor to the first row.
func (t *Table) MoveTop() {
	t.SetCursor(0)
}

// MoveBottom jumps the cursor to the last row.
func (t *Table) MoveBottom() {
	t.SetCursor(len(t.rows) - 1)
}

// NearTop indicates whether the cursor is close to the loaded upper boundary.
func (t *Table) NearTop(threshold int) bool {
	return t.cursor <= threshold
}

// NearBottom indicates whether the cursor is close to the loaded lower boundary.
func (t *Table) NearBottom(threshold int) bool {
	return t.cursor >= len(t.rows)-threshold
}

func (t *Table) adjustOffset() {
	if t.height <= 0 || len(t.rows) == 0 {
		t.offset = 0
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	} else if t.cursor >= t.offset+t.height {
		t.offset = t.cursor - t.height + 1
	}
}

// View renders the visible rows and header of the table.
func (t *Table) View() string {
	var b strings.Builder
	b.WriteString(t.renderHeader())
	b.WriteByte('\n')

	if len(t.rows) == 0 {
		b.WriteString(t.theme.Muted.Render("  (no data)"))
		return b.String()
	}

	end := min(len(t.rows), t.offset+t.height)
	for i := t.offset; i < end; i++ {
		b.WriteString(t.renderRow(i))
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (t *Table) renderHeader() string {
	var cols []string
	if t.showSelect {
		cols = append(cols, "[ ]")
	}
	for _, col := range t.columns {
		title := ansi.Truncate(col.Title, col.Width, "…")
		styled := lipgloss.NewStyle().Width(col.Width).Align(col.Align).Render(title)
		cols = append(cols, styled)
	}
	return t.theme.Header.Render(strings.Join(cols, " "))
}

func (t *Table) renderRow(idx int) string {
	row := t.rows[idx]
	var cols []string
	if t.showSelect {
		mark := "[ ]"
		if t.selected[row.ID] {
			mark = "[x]"
		}
		cols = append(cols, mark)
	}

	for j, col := range t.columns {
		cellVal := ""
		if j < len(row.Cells) {
			cellVal = row.Cells[j]
		}
		truncated := ansi.Truncate(cellVal, col.Width, "…")
		rendered := lipgloss.NewStyle().Width(col.Width).Align(col.Align).Render(truncated)
		cols = append(cols, rendered)
	}

	line := strings.Join(cols, " ")
	if idx == t.cursor {
		return t.theme.Selected.Render(line)
	}
	return line
}
