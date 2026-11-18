package budget

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/tui/theme"
)

func TestTierIcons(t *testing.T) {
	th := theme.Default()
	for _, tc := range []struct {
		pct  int
		icon string
	}{
		{0, "✓"}, {69, "✓"}, {70, "!"}, {89, "!"}, {90, "‼"}, {99, "‼"}, {100, "✗ OVER"}, {150, "✗ OVER"},
	} {
		icon, _ := tier(th, tc.pct)
		if icon != tc.icon {
			t.Errorf("tier(%d) icon = %q, want %q", tc.pct, icon, tc.icon)
		}
		if got := progressBar(th, tc.pct); !strings.HasSuffix(got, tc.icon) {
			t.Errorf("progressBar(%d) = %q, want suffix %q", tc.pct, got, tc.icon)
		}
	}
}
