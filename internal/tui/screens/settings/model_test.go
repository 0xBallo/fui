package settings_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings"
	"github.com/0xballo/fui/internal/tui/theme"
)

// run executes cmd and any cmd that Update returns in reaction, until it settles, returning the
// resulting screen. A tea.BatchMsg is unwrapped and each of its commands is drained in turn,
// mirroring how the real Bubble Tea runtime dispatches a batch.
func run(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				scr = run(t, scr, c)
			}
			return scr
		}
		scr, cmd = scr.Update(msg)
	}
	return scr
}

func sendRunes(scr screens.Screen, s string) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func sendKey(scr screens.Screen, k tea.KeyType) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyMsg{Type: k})
}

// newFixture builds a settings screen over a real app.Service and a *config.Config backed by a
// temporary config file, so saves exercise the real persistence path.
func newFixture(t *testing.T) (*config.Config, screens.Screen) {
	t.Helper()
	fx := screentest.New(t)
	cfg := config.Defaults()
	cfg.ConfigFile = filepath.Join(t.TempDir(), "config.toml")
	if err := cfg.SaveToFile(cfg.ConfigFile); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
	scr := screens.Screen(settings.New(fx.Service, &cfg, &cfg, theme.Default()))
	scr.SetSize(100, 30)
	return &cfg, scr
}

func TestSettingsLoadShowsModulesAndTheme(t *testing.T) {
	_, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	view := scr.View()
	if !strings.Contains(view, "core") || !strings.Contains(view, "yes") {
		t.Fatalf("expected the core module listed as enabled, got:\n%s", view)
	}
	if !strings.Contains(view, "Modules") {
		t.Fatalf("expected the Modules section active, got:\n%s", view)
	}
	if scr.Title() != "Settings" {
		t.Fatalf("expected title %q, got %q", "Settings", scr.Title())
	}
	if len(scr.KeyMap()) == 0 {
		t.Fatalf("expected a non-empty key map")
	}
	if scr.InputActive() {
		t.Fatalf("expected InputActive to be false")
	}
}

func TestSettingsEnableAlreadyEnabledModuleIsNoop(t *testing.T) {
	cfg, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	// "core" is the first row and already enabled; toggling it must be a no-op notice, not a
	// disable (Core cannot be disabled).
	scr, cmd := sendKey(scr, tea.KeyEnter)
	run(t, scr, cmd)

	enabled, err := cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	if !enabled.Has(module.Core) {
		t.Fatalf("expected core to remain enabled, got %+v", enabled.IDs())
	}
}

func TestSettingsDisableConfirmationCanBeCancelled(t *testing.T) {
	cfg, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	for i := 0; i < 4; i++ {
		scr, _ = sendKey(scr, tea.KeyDown)
	}
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	scr, _ = sendKey(scr, tea.KeyDown)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	scr, _ = sendKey(scr, tea.KeyUp)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Disable module") {
		t.Fatalf("expected a confirmation dialog")
	}

	scr, _ = sendRunes(scr, "n")
	if strings.Contains(scr.View(), "Disable module") {
		t.Fatalf("expected the dialog to close on cancel")
	}

	enabled, err := cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	if !enabled.Has(module.Investments) || !enabled.Has(module.Crypto) {
		t.Fatalf("expected both modules to remain enabled after cancel, got %+v", enabled.IDs())
	}
}

func TestSettingsEnableModule(t *testing.T) {
	cfg, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	// "investments" is not enabled by default (see module.registryEntry); move the cursor to it
	// and toggle it on.
	for i := 0; i < 4; i++ {
		scr, _ = sendKey(scr, tea.KeyDown)
	}
	if !strings.Contains(scr.View(), "investments") {
		t.Fatalf("expected the cursor row to be investments, got:\n%s", scr.View())
	}
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	enabled, err := cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	if !enabled.Has(module.Investments) {
		t.Fatalf("expected investments enabled in config, got %+v", enabled.IDs())
	}
	if !strings.Contains(scr.View(), "yes") {
		t.Fatalf("expected the table to reflect the new state, got:\n%s", scr.View())
	}
}

func TestSettingsDisableModuleWithDependentsRequiresConfirmation(t *testing.T) {
	cfg, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	// Enable investments then crypto (which depends on investments).
	for i := 0; i < 4; i++ {
		scr, _ = sendKey(scr, tea.KeyDown)
	}
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	scr, _ = sendKey(scr, tea.KeyDown)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	enabled, err := cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	if !enabled.Has(module.Crypto) || !enabled.Has(module.Investments) {
		t.Fatalf("expected investments and crypto enabled, got %+v", enabled.IDs())
	}

	// Move the cursor back up to investments and try to disable it: crypto still depends on it,
	// so a confirmation dialog must appear instead of an immediate disable.
	scr, _ = sendKey(scr, tea.KeyUp)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	view := scr.View()
	if !strings.Contains(view, "Disable module") {
		t.Fatalf("expected a confirmation dialog, got:\n%s", view)
	}

	enabled, err = cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	if !enabled.Has(module.Investments) {
		t.Fatalf("expected investments still enabled pending confirmation, got %+v", enabled.IDs())
	}

	scr, cmd = sendRunes(scr, "y")
	run(t, scr, cmd)

	enabled, err = cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled: %v", err)
	}
	if enabled.Has(module.Investments) || enabled.Has(module.Crypto) {
		t.Fatalf("expected both investments and crypto disabled, got %+v", enabled.IDs())
	}
}

func TestSettingsApplyTheme(t *testing.T) {
	cfg, scr := newFixture(t)
	scr = run(t, scr, scr.Init())

	scr, _ = sendKey(scr, tea.KeyTab)
	if !strings.Contains(scr.View(), "Theme") {
		t.Fatalf("expected the Appearance section, got:\n%s", scr.View())
	}

	scr, _ = sendKey(scr, tea.KeyDown) // auto -> latte
	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	got, err := cfg.Get("theme")
	if err != nil {
		t.Fatalf("Get(theme): %v", err)
	}
	if got != "latte" {
		t.Fatalf("expected theme %q saved, got %q", "latte", got)
	}
	if !strings.Contains(scr.View(), "current") {
		t.Fatalf("expected the table to mark the new current theme, got:\n%s", scr.View())
	}
}
