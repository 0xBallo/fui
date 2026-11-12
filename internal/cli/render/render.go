// Package render turns use-case results into the three output formats of the CLI: an aligned
// table for interactive use, --json for scripts and agents, and --csv for spreadsheets. It knows
// nothing about cobra or the app use cases; commands build a Table or a JSON-ready value and call
// the Write* function matching the requested Format.
package render

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Format is the requested output format of a command.
type Format int

// Supported formats; Table is the default.
const (
	Table Format = iota
	JSON
	CSV
)

// Rows is an aligned table: Header always prints, Rows holds the formatted cell values. Cells in
// MoneyCols are colored red when they start with "-" and color is enabled, per the FS rule that
// outflows are shown with a minus sign (and, on a terminal, in color).
type Rows struct {
	Header    []string
	Body      [][]string
	MoneyCols []int
}

const ansiRed = "\x1b[31m"

const ansiReset = "\x1b[0m"

// WriteTable prints t as a column-aligned table. Color enables red for negative amounts; it must
// already account for NO_COLOR and for Stdout not being a terminal.
func WriteTable(w io.Writer, t Rows, color bool) error {
	widths := columnWidths(t)
	if err := writeTableRow(w, t.Header, widths, nil, false); err != nil {
		return err
	}
	for _, row := range t.Body {
		if err := writeTableRow(w, row, widths, t.MoneyCols, color); err != nil {
			return err
		}
	}
	return nil
}

func columnWidths(t Rows) []int {
	widths := make([]int, len(t.Header))
	for i, h := range t.Header {
		widths[i] = cellWidth(h)
	}
	for _, row := range t.Body {
		for i, cell := range row {
			if i < len(widths) && cellWidth(cell) > widths[i] {
				widths[i] = cellWidth(cell)
			}
		}
	}
	return widths
}

func writeTableRow(w io.Writer, cells []string, widths []int, moneyCols []int, color bool) error {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		pad := strings.Repeat(" ", widths[i]-cellWidth(cell))
		padded := cell + pad
		if color && containsInt(moneyCols, i) && strings.HasPrefix(cell, "-") {
			padded = ansiRed + cell + ansiReset + pad
		}
		parts[i] = padded
	}
	_, err := fmt.Fprintln(w, strings.TrimRight(strings.Join(parts, "  "), " "))
	if err != nil {
		return fmt.Errorf("write table row: %w", err)
	}
	return nil
}

// cellWidth counts runes so accented descriptions ("Caffè") stay aligned.
// ponytail: East Asian wide runes and emoji count as 1; use a display-width library if they matter.
func cellWidth(s string) int { return utf8.RuneCountInString(s) }

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// WriteCSV prints t as RFC 4180 CSV with the header always present, comma-separated and
// dot-decimal regardless of locale.
func WriteCSV(w io.Writer, t Rows) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(t.Header); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}
	for _, row := range t.Body {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

// WriteJSON prints v (a slice for a list, an object for a detail) with snake_case keys coming
// from the struct tags of the caller's DTOs.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}
