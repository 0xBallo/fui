package lists

import (
	"strings"

	"github.com/0xballo/fui/internal/tui/theme"
)

// equalFold reports whether a and b are the same name, ignoring case (matching domain's
// case-insensitive name comparison).
func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

// staticField renders label/value as a read-only line matching an Autocomplete field's layout,
// for values an edit form displays but cannot submit a change for.
func staticField(th theme.Theme, label, value string) string {
	return th.Header.Render(label) + " " + th.Muted.Render(value+" (read-only)")
}

// withError appends an inline error line to body when err is non-nil, so a failed submit stays
// visible on screen instead of leaving the user on an unchanged view with no explanation.
func withError(th theme.Theme, body string, err error) string {
	if err == nil {
		return body
	}
	return body + "\n\n" + th.Error.Render("Error: "+err.Error())
}
