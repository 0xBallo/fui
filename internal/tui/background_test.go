package tui_test

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestRootFollowsReportedBackground checks that the "auto" theme starts dark without an answer
// and switches flavor when the terminal reports its background.
func TestRootFollowsReportedBackground(t *testing.T) {
	t.Setenv("COLORFGBG", "")
	t.Setenv("COLORTERM", "truecolor")
	const (
		latteText = "38;2;76;79;105"   // #4c4f69
		mochaText = "38;2;205;214;244" // #cdd6f4
	)
	_, m := newRoot(t)
	if v := m.View().Content; !strings.Contains(v, mochaText) {
		t.Fatal("expected the mocha flavor before the terminal answers")
	}
	tests := []struct {
		name string
		bg   color.Color
		want string
	}{
		{"light", color.White, latteText},
		{"dark", color.Black, mochaText},
	}
	for _, tc := range tests {
		m, _ = m.Update(tea.BackgroundColorMsg{Color: tc.bg})
		if v := m.View().Content; !strings.Contains(v, tc.want) {
			t.Errorf("%s background: view lacks %q", tc.name, tc.want)
		}
	}
}
