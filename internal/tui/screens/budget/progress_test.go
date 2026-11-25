package budget

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/theme"
)

func TestTierIcons(t *testing.T) {
	th := theme.Default()
	for _, tc := range []struct {
		pct   int
		icon  string
		style lipgloss.Style
	}{
		{0, "✓", th.Positive},
		{69, "✓", th.Positive},
		{70, "!", th.Warning},
		{89, "!", th.Warning},
		{90, "‼", th.Caution},
		{99, "‼", th.Caution},
		{100, "✗ OVER", th.Error},
		{150, "✗ OVER", th.Error},
	} {
		icon, style := tier(th, tc.pct)
		if icon != tc.icon {
			t.Errorf("tier(%d) icon = %q, want %q", tc.pct, icon, tc.icon)
		}
		if style.GetForeground() != tc.style.GetForeground() {
			t.Errorf("tier(%d) style foreground = %v, want %v", tc.pct, style.GetForeground(), tc.style.GetForeground())
		}
		if got := progressBar(th, tc.pct); !strings.HasSuffix(got, tc.icon) {
			t.Errorf("progressBar(%d) = %q, want suffix %q", tc.pct, got, tc.icon)
		}
	}
}
