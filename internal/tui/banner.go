package tui

import (
	"os"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/tui/theme"
)

const (
	// bannerRows is the height of the ASCII-art header.
	bannerRows = 3
	// bannerMinHeight is the smallest terminal height that gets the full banner; below it the
	// title shrinks to one line in the tab bar.
	bannerMinHeight = 30

	authorName = "0xBallo"
	authorURL  = "https://github.com/0xBallo"
)

// bannerLogo is the hand-drawn "FinanceUI" logo, three rows of half-block glyphs, 35 columns wide.
var bannerLogo = [bannerRows]string{
	"█▀▀ ▄ █▀▄ ▄▀▄ █▀▄ █▀▀ █▀▀ █ █ ▀█▀",
	"█▀▀ █ █ █ █▀█ █ █ █   █▀▀ █ █  █ ",
	"▀   ▀ ▀ ▀ ▀ ▀ ▀ ▀ ▀▀▀ ▀▀▀ ▀▀▀ ▀▀▀",
}

var versionRE = regexp.MustCompile(`^v(\d+)\.(\d+)(\..*)?$`)

// formatVersion shortens a release such as "v0.3.0" to "v0.3"; any other value ("dev", a commit
// hash) is shown as is.
func formatVersion(v string) string {
	if m := versionRE.FindStringSubmatch(v); m != nil {
		return "v" + m[1] + "." + m[2]
	}
	if v == "" {
		return "dev"
	}
	return v
}

// authorLink renders the author name as an OSC 8 hyperlink; on terminals without color support
// (and under the no-color theme) it stays plain text so escape sequences never leak into output.
func authorLink() string {
	if colorprofile.Detect(os.Stdout, os.Environ()) <= colorprofile.Ascii {
		return authorName
	}
	return ansi.SetHyperlink(authorURL) + authorName + ansi.ResetHyperlink()
}

// byline is the "vX.Y by 0xBallo" text.
func byline(version string) string {
	return formatVersion(version) + " by " + authorLink()
}

// viewBanner renders the three-row header: the logo with the byline on its bottom row.
func viewBanner(th theme.Theme, version string) string {
	rows := make([]string, bannerRows)
	for i, l := range bannerLogo {
		rows[i] = logoStyle(th).Render(l)
	}
	rows[bannerRows-1] += "  " + th.Muted.Render(byline(version))
	return strings.Join(rows, "\n")
}

// logoStyle colors the logo with the accent token; theme.Title carries a border and would add rows.
func logoStyle(th theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(th.Tokens.Accent)
}
