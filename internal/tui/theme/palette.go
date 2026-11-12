package theme

import (
	catppuccingo "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss"
)

// tokensFromFlavor maps the Catppuccin palette of f onto FUI's semantic Tokens, following the
// "Token semantici" table in the technical spec: Base/Surface0/Surface1 for backgrounds,
// Text/Subtext0/Overlay0 for text and borders, Mauve for the accent, Green/Red for income and
// expense, Yellow/Red for warning and over, Peach/Teal for pending and reconciled, and the eight
// named hues for chart series.
func tokensFromFlavor(f catppuccingo.Flavor) Tokens {
	c := func(col catppuccingo.Color) lipgloss.Color { return lipgloss.Color(col.Hex) }
	return Tokens{
		Base:      c(f.Base()),
		Surface:   c(f.Surface0()),
		Selection: c(f.Surface1()),
		Text:      c(f.Text()),
		Muted:     c(f.Subtext0()),
		Border:    c(f.Overlay0()),
		Accent:    c(f.Mauve()),

		Income:  c(f.Green()),
		Expense: c(f.Red()),

		Warning: c(f.Yellow()),
		Over:    c(f.Red()),

		Pending:    c(f.Peach()),
		Reconciled: c(f.Teal()),

		Series1: c(f.Blue()),
		Series2: c(f.Sapphire()),
		Series3: c(f.Teal()),
		Series4: c(f.Green()),
		Series5: c(f.Yellow()),
		Series6: c(f.Peach()),
		Series7: c(f.Maroon()),
		Series8: c(f.Mauve()),
	}
}

// flavorTheme returns the fixed theme for one of the four Catppuccin flavors (latte, frappe,
// macchiato, mocha), or ok=false if name isn't one of them.
func flavorTheme(name string) (Theme, bool) {
	f := catppuccingo.Variant(name)
	if f == nil {
		return Theme{}, false
	}
	return fromTokens(name, ensureTextContrast(tokensFromFlavor(f), 4.5)), true
}

// highContrastTheme builds the high-contrast theme: per the technical spec it uses the Crust
// background and Text foreground of Latte (light) or Mocha (dark) — whichever the same
// background detection as "auto" picks — with the flavor's saturated accents for maximum
// distinction between elements.
func highContrastTheme(dark bool) Theme {
	f := catppuccingo.Latte
	if dark {
		f = catppuccingo.Mocha
	}
	c := func(col catppuccingo.Color) lipgloss.Color { return lipgloss.Color(col.Hex) }
	t := Tokens{
		Base:      c(f.Crust()),
		Surface:   c(f.Mantle()),
		Selection: c(f.Surface2()),
		Text:      c(f.Text()),
		Muted:     c(f.Text()),
		Border:    c(f.Text()),
		Accent:    c(f.Mauve()),

		Income:  c(f.Green()),
		Expense: c(f.Red()),

		Warning: c(f.Yellow()),
		Over:    c(f.Red()),

		Pending:    c(f.Peach()),
		Reconciled: c(f.Teal()),

		Series1: c(f.Blue()),
		Series2: c(f.Sapphire()),
		Series3: c(f.Teal()),
		Series4: c(f.Green()),
		Series5: c(f.Yellow()),
		Series6: c(f.Peach()),
		Series7: c(f.Maroon()),
		Series8: c(f.Mauve()),
	}
	return fromTokens("high-contrast", ensureTextContrast(t, 7.0))
}

// terminalTokens maps FUI's semantic tokens onto the standard 16-color xterm ANSI palette, for
// terminals without truecolor/256-color support, or for users who prefer their own terminal color
// scheme (see "Profondità di colore e NO_COLOR" in the technical spec). The hex values are the
// conventional xterm defaults for each ANSI slot, used both to render (lipgloss downsamples them
// to the nearest ANSI color when the terminal profile requires it) and to check WCAG contrast
// deterministically in tests.
func terminalTokens() Tokens {
	const (
		black   = lipgloss.Color("#000000")
		red     = lipgloss.Color("#800000")
		green   = lipgloss.Color("#008000")
		yellow  = lipgloss.Color("#808000")
		blue    = lipgloss.Color("#000080")
		magenta = lipgloss.Color("#800080")
		cyan    = lipgloss.Color("#008080")
		white   = lipgloss.Color("#c0c0c0")

		brightBlack   = lipgloss.Color("#808080")
		brightRed     = lipgloss.Color("#ff0000")
		brightGreen   = lipgloss.Color("#00ff00")
		brightYellow  = lipgloss.Color("#ffff00")
		brightBlue    = lipgloss.Color("#0000ff")
		brightMagenta = lipgloss.Color("#ff00ff")
		brightCyan    = lipgloss.Color("#00ffff")
		brightWhite   = lipgloss.Color("#ffffff")
	)
	return Tokens{
		Base:      black,
		Surface:   black,
		Selection: brightBlack,
		Text:      brightWhite,
		Muted:     white,
		Border:    brightBlack,
		Accent:    brightMagenta,

		Income:  brightGreen,
		Expense: brightRed,

		Warning: brightYellow,
		Over:    brightRed,

		Pending:    yellow,
		Reconciled: cyan,

		Series1: blue,
		Series2: brightBlue,
		Series3: cyan,
		Series4: green,
		Series5: yellow,
		Series6: magenta,
		Series7: red,
		Series8: brightMagenta,
	}
}

// terminalTheme is the fixed "terminal" theme built from terminalTokens.
func terminalTheme() Theme {
	return fromTokens("terminal", ensureTextContrast(terminalTokens(), 4.5))
}

// Default returns a concrete theme for callers and tests that need one without regard to the
// terminal's actual background or color depth (the Mocha flavor).
func Default() Theme {
	th, _ := flavorTheme("mocha")
	return th
}
