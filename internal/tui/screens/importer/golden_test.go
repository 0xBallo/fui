package importer_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

type fixedDark struct{}

func (fixedDark) Dark(time.Duration) (dark, ok bool) { return true, true }

// pinProfile fixes lipgloss's color profile for the test: styles render against the shared
// default renderer, whose profile is cached process-wide (see forceColorProfile in internal/tui).
func pinProfile(t *testing.T, p termenv.Profile) {
	t.Helper()
	r := lipgloss.DefaultRenderer()
	prev := r.ColorProfile()
	r.SetColorProfile(p)
	t.Cleanup(func() { r.SetColorProfile(prev) })
}

// goldenViews renders the three views added for the import review (details, module dialog, CSV
// mapper) at 80x24 and diffs each against its golden file.
func goldenViews(t *testing.T, th theme.Theme) {
	t.Helper()
	_, _, scr := richPreview(t, th, &recordingModules{}, &recordingProfiles{})
	scr, _ = sendKey(scr, tea.KeyTab)
	for range 5 {
		scr, _ = sendRunes(scr, "j")
	}
	scr, _ = sendRunes(scr, "t")
	t.Run("details", func(t *testing.T) { golden.RequireEqual(t, []byte(scr.View())) })

	scr, _ = sendRunes(scr, "m")
	scr, _ = sendRunes(scr, " ")
	t.Run("modules", func(t *testing.T) { golden.RequireEqual(t, []byte(scr.View())) })

	scr = csvMapperScreen(t, th)
	t.Run("csvmap", func(t *testing.T) { golden.RequireEqual(t, []byte(scr.View())) })
}

func csvMapperScreen(t *testing.T, th theme.Theme) screens.Screen {
	t.Helper()
	fx := newFixture()
	m := newImporter(fx, th)
	m.SetModules(module.Default(), nil)
	m.SetProfiles(&recordingProfiles{})
	var scr screens.Screen = m
	scr.SetSize(80, 24)
	scr = run(t, scr, scr.Init())
	scr, _ = sendRunes(scr, "n")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, _ = sendRunes(scr, "bank.csv")
	scr, cmd := sendKey(scr, tea.KeyCtrlP)
	return run(t, scr, cmd)
}

func TestImportGoldens(t *testing.T) {
	for _, name := range []string{"latte", "mocha", "terminal", "high-contrast"} {
		t.Run(name, func(t *testing.T) {
			pinProfile(t, termenv.TrueColor)
			goldenViews(t, theme.Resolve(theme.Config{Theme: name}, fixedDark{}))
		})
	}
	t.Run("no-color", func(t *testing.T) {
		pinProfile(t, termenv.Ascii)
		goldenViews(t, theme.Resolve(theme.Config{Theme: "mocha"}, fixedDark{}))
	})
}
