package theme

import (
	"math"
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

// ensureTextContrast returns t with every token the technical spec treats as "text" (Text, Muted,
// Income, Expense, Warning, Over, Pending, Reconciled) nudged in lightness, if needed, so each
// meets at least min contrast against Base. Catppuccin's flavors aren't all designed for
// text-level contrast on every background (Latte's pastel accents in particular fall short), so
// FUI enforces the technical spec's WCAG requirement programmatically instead of hoping each
// flavor already satisfies it; this keeps every token's hue recognizable while guaranteeing the
// invariant for any flavor, present or future.
func ensureTextContrast(t Tokens, minContrast float64) Tokens {
	t.Text = ensureContrast(t.Base, t.Text, minContrast)
	t.Muted = ensureContrast(t.Base, t.Muted, minContrast)
	t.Income = ensureContrast(t.Base, t.Income, minContrast)
	t.Expense = ensureContrast(t.Base, t.Expense, minContrast)
	t.Warning = ensureContrast(t.Base, t.Warning, minContrast)
	t.Over = ensureContrast(t.Base, t.Over, minContrast)
	t.Pending = ensureContrast(t.Base, t.Pending, minContrast)
	t.Reconciled = ensureContrast(t.Base, t.Reconciled, minContrast)
	return t
}

// ensureContrast returns fg unchanged if it already meets min contrast against base, otherwise
// the closest color on the fg-to-black (if base is light) or fg-to-white (if base is dark) line
// that does, found by bisection since contrast is monotonic along that line.
func ensureContrast(base, fg lipgloss.Color, minContrast float64) lipgloss.Color {
	if contrastRatioHex(base, fg) >= minContrast {
		return fg
	}

	fr, fgc, fb := hexToRGB(fg)
	target := 0.0 // blend toward black
	if luminanceHex(base) < 0.5 {
		target = 1.0 // base is dark: blend toward white instead
	}

	lo, hi := 0.0, 1.0
	for range 40 {
		mid := (lo + hi) / 2
		r := blend(fr, target, mid)
		g := blend(fgc, target, mid)
		b := blend(fb, target, mid)
		if relativeLuminance(r, g, b) == relativeLuminance(fr, fgc, fb) {
			// mid has no effect (shouldn't happen once fr != target), guard against infinite loop
			break
		}
		candidate := rgbToHex(r, g, b)
		if contrastRatioHex(base, candidate) >= minContrast {
			hi = mid
		} else {
			lo = mid
		}
	}
	r := blend(fr, target, hi)
	g := blend(fgc, target, hi)
	b := blend(fb, target, hi)
	return rgbToHex(r, g, b)
}

func blend(c, target, t float64) float64 { return c + (target-c)*t }

// hexToRGB parses a "#RRGGBB" lipgloss.Color into [0,1] RGB components.
func hexToRGB(c lipgloss.Color) (r, g, b float64) {
	s := string(c)
	return hexChannel(s[1:3]), hexChannel(s[3:5]), hexChannel(s[5:7])
}

func hexChannel(s string) float64 {
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0
	}
	return float64(v) / 255
}

// rgbToHex formats [0,1] RGB components (clamped) as a "#RRGGBB" lipgloss.Color.
func rgbToHex(r, g, b float64) lipgloss.Color {
	ch := func(v float64) string {
		v = math.Max(0, math.Min(1, v))
		return padHex(int(math.Round(v * 255)))
	}
	return lipgloss.Color("#" + ch(r) + ch(g) + ch(b))
}

func padHex(v int) string {
	s := strconv.FormatInt(int64(v), 16)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

// luminanceHex and contrastRatioHex wrap relativeLuminance for "#RRGGBB" lipgloss.Colors.
func luminanceHex(c lipgloss.Color) float64 {
	r, g, b := hexToRGB(c)
	return relativeLuminance(r, g, b)
}

func contrastRatioHex(a, b lipgloss.Color) float64 {
	la, lb := luminanceHex(a), luminanceHex(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
