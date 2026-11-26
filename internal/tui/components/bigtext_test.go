package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/go-cmp/cmp"
)

func TestBigTextGolden(t *testing.T) {
	tests := map[string]string{
		"money":  "-1.234.567,89",
		"signed": "+0",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			golden.RequireEqual(t, []byte(BigText(in, lipgloss.NewStyle())+"\n"))
		})
	}
}

func TestBigTextWidth(t *testing.T) {
	for _, in := range []string{"", "0", "-1.234.567,89", "+0", "€5", "1 2", "−'"} {
		out := BigText(in, lipgloss.NewStyle())
		if got, want := BigTextWidth(in), lipgloss.Width(out); got != want {
			t.Errorf("BigTextWidth(%q) = %d, lipgloss.Width = %d", in, got, want)
		}
		if n := strings.Count(out, "\n") + 1; n != bigTextRows {
			t.Errorf("BigText(%q) has %d rows", in, n)
		}
	}
}

func TestBigTextUnknownRune(t *testing.T) {
	rows := strings.Split(BigText("€", lipgloss.NewStyle()), "\n")
	if diff := cmp.Diff([]string{" ", "€", " "}, rows); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestBigTextCommaDiffersFromDot(t *testing.T) {
	if bigGlyphs['.'] == bigGlyphs[','] {
		t.Error("'.' and ',' share the same glyph")
	}
}
