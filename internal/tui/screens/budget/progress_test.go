package budget

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/theme"
)

func TestTierIcons(t *testing.T) {
	th := theme.Default()
	def := domain.BudgetThresholds{}
	for _, tc := range []struct {
		pct  int
		t    domain.BudgetThresholds
		icon string
	}{
		{0, def, "✓"},
		{79, def, "✓"},
		{80, def, "!"},
		{99, def, "!"},
		{100, def, "✗ OVER"},
		{150, def, "✗ OVER"},
		{49, domain.BudgetThresholds{Warn: 50, Over: 120}, "✓"},
		{50, domain.BudgetThresholds{Warn: 50, Over: 120}, "!"},
		{119, domain.BudgetThresholds{Warn: 50, Over: 120}, "!"},
		{120, domain.BudgetThresholds{Warn: 50, Over: 120}, "✗ OVER"},
	} {
		icon, _ := tier(th, tc.pct, tc.t)
		if icon != tc.icon {
			t.Errorf("tier(%d, %+v) icon = %q, want %q", tc.pct, tc.t, icon, tc.icon)
		}
		if got := progressBar(th, tc.pct, tc.t); !strings.HasSuffix(got, tc.icon) {
			t.Errorf("progressBar(%d) = %q, want suffix %q", tc.pct, got, tc.icon)
		}
	}
}
