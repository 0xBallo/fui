// Package theme defines the visual styling and palettes for the TUI, supporting
// standard adaptive terminal palettes and an accessible high-contrast variant.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme encapsulates lipgloss styles for UI elements to ensure consistent
// presentation across screens and components.
type Theme struct {
	Name        string
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
}

// Default provides adaptive colors adjusting to light and dark terminal backgrounds.
func Default() Theme {
	border := lipgloss.AdaptiveColor{Light: "#767676", Dark: "#555555"}
	accent := lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"}
	fg := lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F3F4F6"}
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	pos := lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	neg := lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	warn := lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}
	selBg := lipgloss.AdaptiveColor{Light: "#E0E7FF", Dark: "#1E293B"}

	return Theme{
		Name:        "default",
		Base:        lipgloss.NewStyle().Foreground(fg),
		Title:       lipgloss.NewStyle().Bold(true).Foreground(accent),
		Header:      lipgloss.NewStyle().Bold(true).Foreground(fg),
		ActiveTab:   lipgloss.NewStyle().Bold(true).Foreground(accent).Underline(true),
		InactiveTab: lipgloss.NewStyle().Foreground(muted),
		DisabledTab: lipgloss.NewStyle().Foreground(muted).Faint(true),
		Border:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(border),
		Selected:    lipgloss.NewStyle().Background(selBg).Bold(true),
		Muted:       lipgloss.NewStyle().Foreground(muted),
		Positive:    lipgloss.NewStyle().Foreground(pos),
		Negative:    lipgloss.NewStyle().Foreground(neg),
		Warning:     lipgloss.NewStyle().Foreground(warn),
		Error:       lipgloss.NewStyle().Bold(true).Foreground(neg),
		StatusBar:   lipgloss.NewStyle().Foreground(muted),
		HelpKey:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		HelpDesc:    lipgloss.NewStyle().Foreground(muted),
		Dialog:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(1, 2),
		BarFill:     lipgloss.NewStyle().Foreground(accent),
	}
}

// HighContrast provides maximum contrast for accessibility in constrained lighting or vision needs.
func HighContrast() Theme {
	white := lipgloss.Color("#FFFFFF")
	black := lipgloss.Color("#000000")
	yellow := lipgloss.Color("#FFFF00")
	cyan := lipgloss.Color("#00FFFF")
	green := lipgloss.Color("#00FF00")
	red := lipgloss.Color("#FF5555")

	return Theme{
		Name:        "high-contrast",
		Base:        lipgloss.NewStyle().Foreground(white),
		Title:       lipgloss.NewStyle().Bold(true).Foreground(yellow),
		Header:      lipgloss.NewStyle().Bold(true).Foreground(white).Underline(true),
		ActiveTab:   lipgloss.NewStyle().Bold(true).Foreground(yellow).Reverse(true),
		InactiveTab: lipgloss.NewStyle().Foreground(white),
		DisabledTab: lipgloss.NewStyle().Foreground(white).Faint(true),
		Border:      lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(white),
		Selected:    lipgloss.NewStyle().Background(yellow).Foreground(black).Bold(true),
		Muted:       lipgloss.NewStyle().Foreground(white),
		Positive:    lipgloss.NewStyle().Bold(true).Foreground(green),
		Negative:    lipgloss.NewStyle().Bold(true).Foreground(red),
		Warning:     lipgloss.NewStyle().Bold(true).Foreground(yellow),
		Error:       lipgloss.NewStyle().Bold(true).Background(red).Foreground(white),
		StatusBar:   lipgloss.NewStyle().Foreground(white),
		HelpKey:     lipgloss.NewStyle().Bold(true).Foreground(cyan),
		HelpDesc:    lipgloss.NewStyle().Foreground(white),
		Dialog:      lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(yellow).Padding(1, 2),
		BarFill:     lipgloss.NewStyle().Foreground(cyan),
	}
}

// ByName returns the appropriate theme matching the requested configuration.
func ByName(name string) Theme {
	if name == "high-contrast" {
		return HighContrast()
	}
	return Default()
}
