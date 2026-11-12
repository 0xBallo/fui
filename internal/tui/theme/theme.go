// Package theme defines the visual styling and color palettes for the TUI: the four Catppuccin
// flavors (Latte, Frappé, Macchiato, Mocha), a 16-color "terminal" fallback, and an accessible
// "high-contrast" variant. Components never reference palette colors directly; they render
// through the semantic Tokens this package maps onto each flavor (see "Token semantici" in the
// technical spec), so a new flavor or a palette tweak never touches screen code.
package theme

import "github.com/charmbracelet/lipgloss"

// Tokens are the semantic color roles every theme must define, independent of any particular
// Catppuccin flavor. Screens and components never see Tokens directly: they use the lipgloss
// styles in Theme, which are built from Tokens once per theme.
type Tokens struct {
	// Base, Surface and Selection are background roles: the terminal background, panels, and the
	// highlighted row.
	Base      lipgloss.Color
	Surface   lipgloss.Color
	Selection lipgloss.Color
	// Text, Muted and Border are foreground roles for body text, secondary text, and borders.
	Text   lipgloss.Color
	Muted  lipgloss.Color
	Border lipgloss.Color
	// Accent marks the active screen, focus, and titles.
	Accent lipgloss.Color
	// Income and Expense color inflows and outflows; always paired with a sign, never the only
	// signal.
	Income  lipgloss.Color
	Expense lipgloss.Color
	// Warning and Over mark budgets near or past their threshold; always paired with a label.
	Warning lipgloss.Color
	Over    lipgloss.Color
	// Pending and Reconciled mark transaction states; always paired with a symbol.
	Pending    lipgloss.Color
	Reconciled lipgloss.Color
	// Series1..Series8 color chart and sparkline series.
	Series1 lipgloss.Color
	Series2 lipgloss.Color
	Series3 lipgloss.Color
	Series4 lipgloss.Color
	Series5 lipgloss.Color
	Series6 lipgloss.Color
	Series7 lipgloss.Color
	Series8 lipgloss.Color
}

// Theme encapsulates lipgloss styles for UI elements to ensure consistent presentation across
// screens and components. Every style is built from Tokens so no component needs its own colors.
type Theme struct {
	Name        string
	Tokens      Tokens
	Base        lipgloss.Style
	Title       lipgloss.Style
	Header      lipgloss.Style
	ActiveTab   lipgloss.Style
	InactiveTab lipgloss.Style
	DisabledTab lipgloss.Style
	Border      lipgloss.Style
	Selected    lipgloss.Style
	Muted       lipgloss.Style
	Positive    lipgloss.Style
	Negative    lipgloss.Style
	Warning     lipgloss.Style
	Error       lipgloss.Style
	StatusBar   lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style
	Dialog      lipgloss.Style
	BarFill     lipgloss.Style
	// Button and ActiveButton render a dialog's choices; the active one is the default action.
	Button       lipgloss.Style
	ActiveButton lipgloss.Style
}

// fromTokens builds every Theme style from t, the only place in the package that wires a
// semantic token to a concrete lipgloss property.
func fromTokens(name string, t Tokens) Theme {
	return Theme{
		Name:   name,
		Tokens: t,
		Base:   lipgloss.NewStyle().Foreground(t.Text),
		Title: lipgloss.NewStyle().Bold(true).Foreground(t.Accent).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(t.Border),
		Header:       lipgloss.NewStyle().Bold(true).Foreground(t.Text),
		ActiveTab:    lipgloss.NewStyle().Bold(true).Foreground(t.Accent).Underline(true),
		InactiveTab:  lipgloss.NewStyle().Foreground(t.Muted),
		DisabledTab:  lipgloss.NewStyle().Foreground(t.Muted).Faint(true),
		Border:       lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border),
		Selected:     lipgloss.NewStyle().Background(t.Selection).Bold(true),
		Muted:        lipgloss.NewStyle().Foreground(t.Muted),
		Positive:     lipgloss.NewStyle().Foreground(t.Income),
		Negative:     lipgloss.NewStyle().Foreground(t.Expense),
		Warning:      lipgloss.NewStyle().Foreground(t.Warning),
		Error:        lipgloss.NewStyle().Bold(true).Foreground(t.Expense),
		StatusBar:    lipgloss.NewStyle().Foreground(t.Muted),
		HelpKey:      lipgloss.NewStyle().Bold(true).Foreground(t.Accent),
		HelpDesc:     lipgloss.NewStyle().Foreground(t.Muted),
		Dialog:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Accent).Padding(0, 2),
		BarFill:      lipgloss.NewStyle().Foreground(t.Series1),
		Button:       lipgloss.NewStyle().Foreground(t.Base).Background(t.Muted).Padding(0, 3),
		ActiveButton: lipgloss.NewStyle().Foreground(t.Base).Background(t.Accent).Padding(0, 3).Underline(true),
	}
}
