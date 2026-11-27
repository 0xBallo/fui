package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const bigTextRows = 3

// bigGlyphs holds the 3-row block glyphs; every row of a glyph has the same width.
var bigGlyphs = map[rune][bigTextRows]string{
	'0':  {"█▀█", "█ █", "▀▀▀"},
	'1':  {"▄█ ", " █ ", "▀▀▀"},
	'2':  {"▀▀█", "█▀▀", "▀▀▀"},
	'3':  {"▀▀█", " ▀█", "▀▀▀"},
	'4':  {"█ █", "▀▀█", "  ▀"},
	'5':  {"█▀▀", "▀▀█", "▀▀▀"},
	'6':  {"█▀▀", "█▀█", "▀▀▀"},
	'7':  {"▀▀█", "  █", "  ▀"},
	'8':  {"█▀█", "█▀█", "▀▀▀"},
	'9':  {"█▀█", "▀▀█", "▀▀▀"},
	'.':  {" ", " ", "▄"},
	',':  {" ", "▄", "▀"},
	'\'': {"▀", " ", " "},
	'-':  {"   ", "▀▀▀", "   "},
	'−':  {"   ", "▀▀▀", "   "},
	'+':  {" █ ", "▀█▀", "   "},
	' ':  {"  ", "  ", "  "},
}

// BigText renders s with 3-row block glyphs; runes without a glyph render as normal text, vertically centered.
func BigText(s string, style lipgloss.Style) string {
	var rows [bigTextRows]string
	first := true
	for _, r := range s {
		if !first {
			for i := range rows {
				rows[i] += " "
			}
		}
		first = false
		if g, ok := bigGlyphs[r]; ok {
			for i := range rows {
				rows[i] += g[i]
			}
			continue
		}
		pad := strings.Repeat(" ", lipgloss.Width(string(r)))
		for i := range rows {
			if i == bigTextRows/2 {
				rows[i] += string(r)
			} else {
				rows[i] += pad
			}
		}
	}
	out := make([]string, bigTextRows)
	for i := range rows {
		out[i] = style.Render(rows[i])
	}
	return strings.Join(out, "\n")
}

// BigTextWidth is the rendered width of s, so callers can fall back to normal text when it does not fit.
func BigTextWidth(s string) int {
	w, n := 0, 0
	for _, r := range s {
		if g, ok := bigGlyphs[r]; ok {
			w += lipgloss.Width(g[0])
		} else {
			w += lipgloss.Width(string(r))
		}
		n++
	}
	if n > 1 {
		w += n - 1
	}
	return w
}
