package settings

// themeNames lists the theme picker's choices, matching internal/config's theme constants
// ("auto", the four Catppuccin flavors, "terminal", and "high-contrast") in display order; kept
// local so this package does not import internal/config.
var themeNames = []string{"auto", "latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"}
