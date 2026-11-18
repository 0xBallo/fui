package components

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-cmp/cmp"
)

var updateBigText = flag.Bool("update", false, "rewrite golden files")

func TestBigTextGolden(t *testing.T) {
	tests := map[string]string{
		"money":  "-1.234.567,89",
		"signed": "+0",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			got := BigText(in, lipgloss.NewStyle()) + "\n"
			path := filepath.Join("testdata", "bigtext_"+name+".golden")
			if *updateBigText {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
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
