package theme_test

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/tui/theme"
)

// allThemes returns every theme the technical spec requires: the four Catppuccin flavors,
// "terminal", and both backgrounds of "high-contrast" (light and dark pick different flavors).
func allThemes(t *testing.T) map[string]theme.Theme {
	t.Helper()
	out := map[string]theme.Theme{}
	for _, name := range []string{"latte", "frappe", "macchiato", "mocha", "terminal"} {
		out[name] = theme.Resolve(theme.Config{Theme: name}, nil)
	}
	out["high-contrast (light)"] = theme.Resolve(theme.Config{Theme: "high-contrast"}, fakeDetector{ok: true, dark: false})
	out["high-contrast (dark)"] = theme.Resolve(theme.Config{Theme: "high-contrast"}, fakeDetector{ok: true, dark: true})
	return out
}

// TestDefault checks that Default returns the Mocha flavor theme, the fallback several screens'
// own tests rely on when they don't care which theme is active.
func TestDefault(t *testing.T) {
	got := theme.Default()
	if got.Name != "mocha" {
		t.Errorf("Default().Name = %q, want mocha", got.Name)
	}
}

// TestEveryThemeDefinesEveryToken fails if a token was added to Tokens without mapping it in
// every flavor: every field must be a non-empty lipgloss.Color.
func TestEveryThemeDefinesEveryToken(t *testing.T) {
	for name, th := range allThemes(t) {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(th.Tokens)
			for i := 0; i < v.NumField(); i++ {
				field := v.Type().Field(i)
				color, ok := v.Field(i).Interface().(lipgloss.Color)
				if !ok {
					t.Fatalf("token %s is not a lipgloss.Color", field.Name)
				}
				if color == "" {
					t.Errorf("theme %q: token %s is undefined", name, field.Name)
				}
			}
		})
	}
}

// textTokens are the tokens the technical spec requires to meet a WCAG contrast ratio against
// Base: at least 4.5:1 for every theme, and at least 7:1 for "high-contrast".
var textTokens = []string{"Text", "Muted", "Income", "Expense", "Warning", "Over", "Pending", "Reconciled"}

// TestContrastWCAG checks every text token of every theme against Base, failing if any token
// drops below the required ratio.
func TestContrastWCAG(t *testing.T) {
	for name, th := range allThemes(t) {
		minRatio := 4.5
		if name == "high-contrast (light)" || name == "high-contrast (dark)" {
			minRatio = 7.0
		}
		t.Run(name, func(t *testing.T) {
			base := th.Tokens.Base
			tokens := reflect.ValueOf(th.Tokens)
			for _, field := range textTokens {
				fg := tokens.FieldByName(field).Interface().(lipgloss.Color)
				ratio := contrastRatio(base, fg)
				if ratio < minRatio {
					t.Errorf("theme %q: token %s contrast with Base = %.2f, want >= %.1f", name, field, ratio, minRatio)
				}
			}
		})
	}
}

// contrastRatio computes the WCAG contrast ratio between two hex lipgloss colors.
func contrastRatio(a, b lipgloss.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance computes the WCAG relative luminance of a "#RRGGBB" lipgloss.Color.
func luminance(c lipgloss.Color) float64 {
	hex := string(c)
	r := hexChannel(hex[1:3])
	g := hexChannel(hex[3:5])
	b := hexChannel(hex[5:7])
	lin := func(ch float64) float64 {
		if ch <= 0.03928 {
			return ch / 12.92
		}
		return math.Pow((ch+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func hexChannel(s string) float64 {
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		panic(fmt.Sprintf("invalid hex channel %q: %v", s, err))
	}
	return float64(v) / 255
}
