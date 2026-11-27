package screentest

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// Downsample converts the truecolor output of a view to profile p, as Bubble Tea's renderer does
// on a terminal with that profile (lipgloss v2 always renders truecolor and leaves the conversion
// to the writer). colorprofile.Ascii is what NO_COLOR selects.
func Downsample(s string, p colorprofile.Profile) string {
	var b strings.Builder
	w := &colorprofile.Writer{Forward: &b, Profile: p}
	_, _ = w.WriteString(s)
	return b.String()
}

var sgrRE = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// HasColor reports whether s sets a foreground, background or underline color in any SGR
// sequence. Text attributes such as bold or underline are not colors: NO_COLOR keeps them.
func HasColor(s string) bool {
	for _, m := range sgrRE.FindAllStringSubmatch(s, -1) {
		params := strings.FieldsFunc(m[1], func(r rune) bool { return r == ';' })
		for _, p := range params {
			p, _, _ = strings.Cut(p, ":")
			n, err := strconv.Atoi(p)
			if err != nil {
				continue
			}
			if (n >= 30 && n <= 38) || (n >= 40 && n <= 48) || (n >= 90 && n <= 97) || (n >= 100 && n <= 107) || n == 58 {
				return true
			}
		}
	}
	return false
}
