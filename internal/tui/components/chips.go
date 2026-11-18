package components

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/tui/theme"
)

// Chips renders labels as space-separated tag chips, truncated as a whole to maxWidth cells with
// an ellipsis. A maxWidth below 1 yields an empty string.
func Chips(th theme.Theme, labels []string, maxWidth int) string {
	if maxWidth < 1 || len(labels) == 0 {
		return ""
	}
	chips := make([]string, len(labels))
	for i, l := range labels {
		chips[i] = th.Chip.Render(l)
	}
	return ansi.Truncate(strings.Join(chips, " "), maxWidth, "…")
}
