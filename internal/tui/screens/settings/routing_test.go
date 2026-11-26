package settings_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings"
	"github.com/0xballo/fui/internal/tui/theme"
)

// TestSettingsSectionResultSurvivesSectionSwitch starts the Categories load, moves to Tags before
// the result arrives, and checks the result still lands in Categories.
func TestSettingsSectionResultSurvivesSectionSwitch(t *testing.T) {
	fx := screentest.New(t)
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: string(domain.CategoryExpense)})
	cfg := config.Defaults()
	cfg.ConfigFile = filepath.Join(t.TempDir(), "config.toml")
	if err := cfg.SaveToFile(cfg.ConfigFile); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
	m := settings.New(fx.Service, &cfg, &cfg, theme.Default())
	m.SetLists(settings.Lists{Categories: fx.Service, Tags: fx.Service})
	m.SetSize(100, 30)
	var scr screens.Screen = m
	scr = run(t, scr, scr.Init())

	scr, _ = sendKey(scr, tea.KeyTab) // Appearance
	scr, pending := sendKey(scr, tea.KeyTab)
	if pending == nil {
		t.Fatal("entering Categories should start its load")
	}
	scr, _ = sendKey(scr, tea.KeyTab) // Tags, before the Categories result is delivered
	scr = run(t, scr, pending)

	scr, _ = sendKey(scr, tea.KeyTab) // Modules
	scr, _ = sendKey(scr, tea.KeyTab) // Appearance
	scr, _ = sendKey(scr, tea.KeyTab) // Categories again, its own reload deliberately not run
	if !strings.Contains(scr.View(), "Groceries") {
		t.Fatalf("Categories result was dropped after switching section:\n%s", scr.View())
	}
}
