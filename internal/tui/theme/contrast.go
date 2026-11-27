package theme

import (
	"image/color"
	"math"
	"strconv"

	"charm.land/lipgloss/v2"
)

// ensureTextContrast returns t with every token the technical spec treats as "text" (Text, Muted,
// Income, Expense, Warning, Caution, Info, Over, Pending, Reconciled, IncomeSoft, ExpenseSoft) nudged in lightness, if needed, so each
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
	t.Caution = ensureContrast(t.Base, t.Caution, minContrast)
	t.Info = ensureContrast(t.Base, t.Info, minContrast)
	t.IncomeSoft = ensureContrast(t.Base, t.IncomeSoft, minContrast)
	t.ExpenseSoft = ensureContrast(t.Base, t.ExpenseSoft, minContrast)
	t.Over = ensureContrast(t.Base, t.Over, minContrast)
	t.Pending = ensureContrast(t.Base, t.Pending, minContrast)
	t.Reconciled = ensureContrast(t.Base, t.Reconciled, minContrast)
	return t
}

// ensureContrast returns fg unchanged if it already meets min contrast against base, otherwise
// the closest color on the fg-to-black (if base is light) or fg-to-white (if base is dark) line
// that does, found by bisection since contrast is monotonic along that line.
func ensureContrast(base, fg color.Color, minContrast float64) color.Color {
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

// hexToRGB returns the [0,1] RGB components of c.
func hexToRGB(c color.Color) (r, g, b float64) {
	cr, cg, cb, _ := c.RGBA()
	return float64(cr) / 0xffff, float64(cg) / 0xffff, float64(cb) / 0xffff
}

// rgbToHex formats [0,1] RGB components (clamped) as a "#RRGGBB" color.Color.
func rgbToHex(r, g, b float64) color.Color {
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

// luminanceHex and contrastRatioHex wrap relativeLuminance for "#RRGGBB" color.Colors.
func luminanceHex(c color.Color) float64 {
	r, g, b := hexToRGB(c)
	return relativeLuminance(r, g, b)
}

func contrastRatioHex(a, b color.Color) float64 {
	la, lb := luminanceHex(a), luminanceHex(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
