package theme

import (
	"os"
	"strings"
	"time"
)

// oscTimeout is the maximum time Resolve waits for a terminal's OSC 11 reply, chosen to stay
// comfortably inside the 300 ms startup budget the technical spec sets for the TUI.
const oscTimeout = 100 * time.Millisecond

// Config carries the user's theme choice, decoupled from internal/config so this package stays
// free of the rest of FUI's dependency graph.
type Config struct {
	// Theme selects the active theme: "auto" (default), one of the four Catppuccin flavors
	// ("latte", "frappe", "macchiato", "mocha"), "terminal", or "high-contrast".
	Theme string
	// ThemeLight and ThemeDark are the flavors "auto" picks on a light/dark background. Empty
	// defaults to "latte" and "mocha".
	ThemeLight string
	ThemeDark  string
}

// Resolve picks the Theme for cfg: a fixed Catppuccin flavor or "terminal" is used as-is; "auto"
// detects the terminal's background via det (OSC 11, then COLORFGBG, then assumes dark) and picks
// ThemeLight or ThemeDark; "high-contrast" uses the same detection to choose between a Latte- and
// a Mocha-based high-contrast theme. Regardless of the chosen theme, a terminal limited to 16
// colors (or fewer) downgrades to "terminal", per "Profondità di colore" in the technical spec;
// NO_COLOR itself needs no special case here, since lipgloss's own renderer already suppresses
// all color output when it's set.
func Resolve(cfg Config, det Detector) Theme {
	name := strings.ToLower(strings.TrimSpace(cfg.Theme))

	var th Theme
	switch name {
	case "", "auto":
		th = autoTheme(cfg, det)
	case "high-contrast":
		th = highContrastTheme(detectDark(det))
	case "terminal":
		th = terminalTheme()
	default:
		if flavor, ok := flavorTheme(name); ok {
			th = flavor
		} else {
			th = autoTheme(cfg, det)
		}
	}

	if lowColorProfile() {
		return terminalTheme()
	}
	return th
}

// autoTheme resolves cfg's ThemeLight/ThemeDark flavor (defaulting to latte/mocha) based on the
// detected background.
func autoTheme(cfg Config, det Detector) Theme {
	light := strings.ToLower(strings.TrimSpace(cfg.ThemeLight))
	if light == "" {
		light = "latte"
	}
	dark := strings.ToLower(strings.TrimSpace(cfg.ThemeDark))
	if dark == "" {
		dark = "mocha"
	}

	name := light
	if detectDark(det) {
		name = dark
	}
	if th, ok := flavorTheme(name); ok {
		return th
	}
	// An invalid configured flavor was already rejected by internal/config's validation; this is
	// only reached by callers that skip it (e.g. tests), so fall back to the spec's defaults.
	fallback, _ := flavorTheme("latte")
	return fallback
}

// detectDark runs the full background-detection fallback chain: OSC 11 via det, then COLORFGBG,
// then assumes a dark background.
func detectDark(det Detector) bool {
	if det != nil {
		if dark, ok := det.Dark(oscTimeout); ok {
			return dark
		}
	}
	if v := os.Getenv("COLORFGBG"); v != "" {
		if dark, ok := colorFGBGDark(v); ok {
			return dark
		}
	}
	return true
}

// lowColorProfile reports whether the terminal, per TERM/COLORTERM, supports at most the 16 ANSI
// colors: FUI then uses the "terminal" theme instead of downsampling a Catppuccin flavor's
// truecolor hexes. This mirrors the environment-variable portion of termenv's own color-profile
// heuristic, but without termenv's TTY check, since that always collapses to the Ascii profile
// for a non-interactive process (such as under "go test"), making it untestable here. NO_COLOR
// needs no special case: lipgloss already renders no color in that case regardless of which
// theme's tokens populate the styles.
func lowColorProfile() bool {
	colorTerm := strings.ToLower(os.Getenv("COLORTERM"))
	switch colorTerm {
	case "24bit", "truecolor", "yes", "true":
		return false
	}

	term := os.Getenv("TERM")
	switch term {
	case "linux", "xterm":
		return true
	}
	if strings.Contains(term, "256color") {
		return false
	}
	if strings.Contains(term, "color") || strings.Contains(term, "ansi") {
		return true
	}
	return false
}
