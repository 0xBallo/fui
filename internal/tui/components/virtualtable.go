// Package components provides domain-agnostic UI building blocks for the TUI.
package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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

// SetHeight sets the lines the table may use; three go to its borders and header separator, the
// rest (at least one) to visible rows.
func (t *Table) SetHeight(h int) {
	if h > 0 {
		t.height = max(1, h-tableChrome)
		t.adjustOffset()
	}
}

// tableChrome is the number of lines View adds beyond header and rows: top border, header
// separator and bottom border.
const tableChrome = 3

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

// View renders the header and the visible rows inside a rounded lipgloss table. Only the rows in
// the viewport are handed to lipgloss, so rendering cost stays independent of the row count.
func (t *Table) View() string {
	var cols []Column
	if t.showSelect {
		cols = append(cols, Column{Title: "[ ]", Width: 3, Align: lipgloss.Left})
	}
	cols = append(cols, t.columns...)

	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = ansi.Truncate(col.Title, col.Width, "…")
	}
	end := min(len(t.rows), t.offset+t.height)
	rows := make([][]string, 0, end-t.offset)
	for i := t.offset; i < end; i++ {
		rows = append(rows, t.cells(t.rows[i], cols))
	}

	cell := func(col Column) lipgloss.Style { return lipgloss.NewStyle().Width(col.Width).Align(col.Align) }
	out := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(t.theme.Tokens.Border)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return t.theme.Header.Inherit(cell(cols[col]))
			case t.offset+row == t.cursor:
				return t.theme.Selected.Inherit(cell(cols[col]))
			}
			return cell(cols[col])
		}).
		Render()
	if len(t.rows) == 0 {
		out += "\n" + t.theme.Muted.Render("  (no data)")
	}
	return out
}

// cells returns row's cell texts for cols, truncated to each column's width.
func (t *Table) cells(row Row, cols []Column) []string {
	out := make([]string, 0, len(cols))
	if t.showSelect {
		mark := "[ ]"
		if t.selected[row.ID] {
			mark = "[x]"
		}
		out = append(out, mark)
		cols = cols[1:]
	}
	for j, col := range cols {
		v := ""
		if j < len(row.Cells) {
			v = row.Cells[j]
		}
		out = append(out, ansi.Truncate(v, col.Width, "…"))
	}
	return out
}
