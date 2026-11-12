package theme_test

import (
	"testing"
	"time"

	"github.com/0xballo/fui/internal/tui/theme"
)

// fakeDetector simulates a Detector's three possible outcomes: a dark answer, a light answer, or
// no answer within the timeout (ok=false), without touching a real terminal.
type fakeDetector struct {
	dark bool
	ok   bool
}

func (f fakeDetector) Dark(time.Duration) (dark, ok bool) { return f.dark, f.ok }

func TestResolveFixedThemes(t *testing.T) {
	tests := []struct {
		name  string
		theme string
		want  string
	}{
		{name: "latte", theme: "latte", want: "latte"},
		{name: "frappe", theme: "frappe", want: "frappe"},
		{name: "macchiato", theme: "macchiato", want: "macchiato"},
		{name: "mocha", theme: "mocha", want: "mocha"},
		{name: "terminal", theme: "terminal", want: "terminal"},
		{name: "case insensitive", theme: "MOCHA", want: "mocha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := theme.Resolve(theme.Config{Theme: tt.theme}, nil)
			if got.Name != tt.want {
				t.Errorf("Resolve(%q).Name = %q, want %q", tt.theme, got.Name, tt.want)
			}
		})
	}
}

func TestResolveAutoUsesDetector(t *testing.T) {
	tests := []struct {
		name string
		det  fakeDetector
		want string
	}{
		{name: "detector reports light", det: fakeDetector{ok: true, dark: false}, want: "latte"},
		{name: "detector reports dark", det: fakeDetector{ok: true, dark: true}, want: "mocha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := theme.Resolve(theme.Config{Theme: "auto"}, tt.det)
			if got.Name != tt.want {
				t.Errorf("Resolve(auto).Name = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestResolveAutoCustomFlavors(t *testing.T) {
	got := theme.Resolve(theme.Config{
		Theme: "auto", ThemeLight: "frappe", ThemeDark: "macchiato",
	}, fakeDetector{ok: true, dark: true})
	if got.Name != "macchiato" {
		t.Errorf("Resolve(auto, dark).Name = %q, want macchiato", got.Name)
	}

	got = theme.Resolve(theme.Config{
		Theme: "auto", ThemeLight: "frappe", ThemeDark: "macchiato",
	}, fakeDetector{ok: true, dark: false})
	if got.Name != "frappe" {
		t.Errorf("Resolve(auto, light).Name = %q, want frappe", got.Name)
	}
}

// TestResolveFallsBackToColorFGBG checks that an OSC 11 timeout (ok=false) falls back to the
// COLORFGBG environment variable, and that the absence of both defaults to dark.
func TestResolveFallsBackToColorFGBG(t *testing.T) {
	tests := []struct {
		name      string
		colorfgbg string
		want      string
	}{
		{name: "light background", colorfgbg: "0;15", want: "latte"},
		{name: "dark background", colorfgbg: "15;0", want: "mocha"},
		{name: "unset defaults to dark", colorfgbg: "", want: "mocha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLORFGBG", tt.colorfgbg)
			got := theme.Resolve(theme.Config{Theme: "auto"}, fakeDetector{ok: false})
			if got.Name != tt.want {
				t.Errorf("Resolve(auto).Name = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestResolveEmptyThemeDefaultsToAuto(t *testing.T) {
	got := theme.Resolve(theme.Config{}, fakeDetector{ok: true, dark: true})
	if got.Name != "mocha" {
		t.Errorf("Resolve(empty).Name = %q, want mocha", got.Name)
	}
}

func TestResolveUnknownThemeFallsBackToAuto(t *testing.T) {
	got := theme.Resolve(theme.Config{Theme: "solarized"}, fakeDetector{ok: true, dark: false})
	if got.Name != "latte" {
		t.Errorf("Resolve(solarized).Name = %q, want latte", got.Name)
	}
}

func TestResolveHighContrast(t *testing.T) {
	light := theme.Resolve(theme.Config{Theme: "high-contrast"}, fakeDetector{ok: true, dark: false})
	if light.Name != "high-contrast" {
		t.Errorf("Resolve(high-contrast, light).Name = %q, want high-contrast", light.Name)
	}
	dark := theme.Resolve(theme.Config{Theme: "high-contrast"}, fakeDetector{ok: true, dark: true})
	if dark.Name != "high-contrast" {
		t.Errorf("Resolve(high-contrast, dark).Name = %q, want high-contrast", dark.Name)
	}
	// The two backgrounds must use genuinely different tokens (Latte's Crust vs Mocha's Crust).
	if light.Tokens.Base == dark.Tokens.Base {
		t.Errorf("Resolve(high-contrast) light and dark share the same Base token: %v", light.Tokens.Base)
	}
}

// TestResolveLowColorProfileForcesTerminal checks that a terminal limited to 16 colors uses the
// "terminal" theme regardless of the requested theme, while a richer one keeps the request.
func TestResolveLowColorProfileForcesTerminal(t *testing.T) {
	tests := []struct {
		name      string
		colorTerm string
		term      string
		want      string
	}{
		{name: "TERM=xterm, no COLORTERM", term: "xterm", want: "terminal"},
		{name: "TERM=linux", term: "linux", want: "terminal"},
		{name: "TERM contains color", term: "rxvt-color", want: "terminal"},
		{name: "TERM contains 256color", term: "xterm-256color", want: "mocha"},
		{name: "COLORTERM=truecolor", term: "xterm", colorTerm: "truecolor", want: "mocha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLORTERM", tt.colorTerm)
			t.Setenv("NO_COLOR", "")
			t.Setenv("TERM", tt.term)
			got := theme.Resolve(theme.Config{Theme: "mocha"}, nil)
			if got.Name != tt.want {
				t.Errorf("Resolve(mocha) under TERM=%q COLORTERM=%q = %q, want %q", tt.term, tt.colorTerm, got.Name, tt.want)
			}
		})
	}
}
