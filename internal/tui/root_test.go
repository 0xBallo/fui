package tui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens/budget"
	"github.com/0xballo/fui/internal/tui/screens/importer"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings"
)

// useCases composes a Fixture's Service with its reporting use cases to satisfy tui.UseCases.
type useCases struct {
	*app.Service
	screentest.Reporting
}

func run(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		m, cmd = m.Update(msg)
	}
	return m
}

func sendRunes(m tea.Model, s string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func newRoot(t *testing.T) (screentest.Fixture, tea.Model) {
	t.Helper()
	fx := screentest.New(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	root := tui.New(uc, tui.Config{Locale: domain.LocaleENUS})
	m, _ := root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return fx, m
}

// TestNoKeyCollisions verifies that neither Root's own keymap nor any MVP screen's keymap binds
// the same key twice, and that no screen rebinds one of Root's global keys (which would be
// unreachable while that screen is not capturing text input).
func TestNoKeyCollisions(t *testing.T) {
	_, m := newRoot(t)
	root := m.(*tui.Root)

	global := tui.DefaultKeyMap().Bindings()
	assertNoDuplicates(t, "root", global)

	for _, name := range []string{"Dashboard", "Transactions", "Accounts"} {
		scr, _ := root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(screenSwitchKey(name))})
		r := scr.(*tui.Root)
		bindings := activeScreenKeyMap(t, r, name)
		assertNoDuplicates(t, name, bindings)
		assertNoOverlap(t, name, global, bindings)
	}
}

func screenSwitchKey(name string) string {
	switch name {
	case "Dashboard":
		return "1"
	case "Transactions":
		return "2"
	default:
		return "3"
	}
}

func activeScreenKeyMap(t *testing.T, r *tui.Root, name string) []key.Binding {
	t.Helper()
	view := r.View()
	if !strings.Contains(view, name) {
		t.Fatalf("expected the %s tab to be active, got:\n%s", name, view)
	}
	return r.ActiveKeyMap()
}

func assertNoDuplicates(t *testing.T, label string, bindings []key.Binding) {
	t.Helper()
	seen := map[string]bool{}
	for _, b := range bindings {
		for _, k := range b.Keys() {
			if seen[k] {
				t.Fatalf("%s: key %q is bound twice", label, k)
			}
			seen[k] = true
		}
	}
}

func assertNoOverlap(t *testing.T, label string, global, local []key.Binding) {
	t.Helper()
	globalKeys := map[string]bool{}
	for _, b := range global {
		for _, k := range b.Keys() {
			globalKeys[k] = true
		}
	}
	for _, b := range local {
		for _, k := range b.Keys() {
			if globalKeys[k] {
				t.Fatalf("%s: key %q collides with a root global binding", label, k)
			}
		}
	}
}

func TestRootNavigatesTabs(t *testing.T) {
	fx, m := newRoot(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})

	m = run(t, m, m.Init())
	if !strings.Contains(m.View(), "Dashboard") {
		t.Fatalf("expected the dashboard tab active at start, got:\n%s", m.View())
	}

	m, cmd := sendRunes(m, "3")
	m = run(t, m, cmd)
	view := m.View()
	if !strings.Contains(view, "Checking") {
		t.Fatalf("expected the accounts screen after switching to tab 3, got:\n%s", view)
	}
}

func TestRootQuitsOnQ(t *testing.T) {
	_, m := newRoot(t)
	m = run(t, m, m.Init())

	_, cmd := sendRunes(m, "q")
	if cmd == nil {
		t.Fatal("expected a quit command on q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %#v", msg)
	}
}

func TestRootMinimumSizeWarning(t *testing.T) {
	_, m := newRoot(t)
	m = run(t, m, m.Init())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Fatalf("expected a minimum-size warning, got:\n%s", view)
	}
}

func TestRootHelpOverlay(t *testing.T) {
	_, m := newRoot(t)
	m = run(t, m, m.Init())

	m, _ = sendRunes(m, "?")
	view := m.View()
	if !strings.Contains(view, "quit") {
		t.Fatalf("expected the help overlay listing global bindings, got:\n%s", view)
	}

	m, _ = sendRunes(m, "?")
	if strings.Contains(m.View(), "Help —") {
		t.Fatalf("expected the help overlay to close, got:\n%s", m.View())
	}
}

// routedConfig wires every optional screen with nil-embedding stubs: the tests below only route and
// render, they never run a screen's Init command.
func routedConfig(mods ...module.ID) tui.Config {
	set, err := module.Validate(append([]module.ID{module.Core}, mods...))
	if err != nil {
		panic(err)
	}
	return tui.Config{
		Locale: domain.LocaleENUS, Modules: set,
		Budget: struct{ budget.UseCases }{}, Report: struct{ report.UseCases }{},
		Importer: struct{ importer.UseCases }{}, Settings: struct{ settings.UseCases }{},
		ModuleConfig: struct{ settings.ModuleConfig }{}, ThemeConfig: struct{ settings.ThemeConfig }{},
	}
}

func newRoutedRoot(t *testing.T, cfg tui.Config) tea.Model {
	t.Helper()
	fx := screentest.New(t)
	root := tui.New(useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}, cfg)
	m, _ := root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func TestModuleMatrixRoutesBudgetTab(t *testing.T) {
	m := newRoutedRoot(t, routedConfig(module.Budget))
	m, _ = sendRunes(m, "4")
	if got := m.(*tui.Root).ActiveKeyMap(); len(got) == 0 {
		t.Fatal("budget tab should be active with the budget module enabled")
	}

	set, err := module.Validate([]module.ID{module.Core})
	if err != nil {
		t.Fatal(err)
	}
	m, _ = m.Update(settings.ModulesChangedMsg{Enabled: set})
	if !strings.Contains(m.View(), "4 Budget") {
		t.Fatalf("budget tab should stay listed (disabled): %s", m.View())
	}
	m, _ = sendRunes(m, "4")
	if !strings.Contains(m.View(), "not available") {
		t.Fatalf("disabled budget tab must not be routed: %s", m.View())
	}
}

func TestSevenFixedKeysRoute(t *testing.T) {
	m := newRoutedRoot(t, routedConfig(module.Budget))
	for _, k := range []string{"1", "2", "3", "4", "5", "6", "7"} {
		m, _ = sendRunes(m, k)
		if strings.Contains(m.View(), "not available") {
			t.Fatalf("key %s not routed with every screen wired", k)
		}
	}
}

func TestThemeMessagesKeepSettingsScreen(t *testing.T) {
	m := newRoutedRoot(t, routedConfig())
	m, _ = sendRunes(m, "7")
	for _, msg := range []tea.Msg{settings.ThemePreviewMsg{Theme: "mocha"}, settings.ThemeChangedMsg{Theme: "latte"}} {
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		if cmd != nil {
			t.Fatalf("%T must not reload the screen (it would reset the cursor)", msg)
		}
		if m.(*tui.Root).ActiveKeyMap() == nil || strings.Contains(m.View(), "not available") {
			t.Fatalf("settings tab should survive %T", msg)
		}
	}
}

func click(m tea.Model, b tea.MouseButton, x, y int) (tea.Model, tea.Cmd) {
	return m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: b})
}

// TestRootMouse checks that a click on a tab switches to it and that the wheel moves the active
// screen's cursor, here observed through which transaction the delete prompt names.
func TestRootMouse(t *testing.T) {
	fx, m := newRoot(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-1.00", Description: "Older", Date: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-2.00", Description: "Newer", Date: "2026-01-02"})
	m = run(t, m, m.Init())

	// At 100 columns each tab is "N Name" plus padding and borders; "1 Dashboard" spans 0-14.
	m, cmd := click(m, tea.MouseButtonLeft, 20, 1)
	m = run(t, m, cmd)
	if !strings.Contains(m.View(), "Description") {
		t.Fatalf("expected a click on tab 2 to open Transactions, got:\n%s", m.View())
	}

	m, _ = click(m, tea.MouseButtonWheelDown, 40, 10)
	m, _ = sendRunes(m, "d")
	if !strings.Contains(m.View(), `"Older"`) {
		t.Fatalf("expected the wheel to move the cursor to the second row, got:\n%s", m.View())
	}
}

// TestRootHelpModal checks that the help opens as a modal that fits the terminal, wrapping into
// columns at the minimum size, and that a click closes it.
func TestRootHelpModal(t *testing.T) {
	_, m := newRoot(t)
	m = run(t, m, m.Init())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = sendRunes(m, "2")
	m, _ = sendRunes(m, "?")
	view := m.View()
	if !strings.Contains(view, "╭") || !strings.Contains(view, "Help — Transactions") || !strings.Contains(view, "bulk categorize") {
		t.Fatalf("expected the help modal with every binding, got:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines != 24 {
		t.Fatalf("help view is %d lines, want 24 (the terminal height):\n%s", lines, view)
	}
	m, _ = click(m, tea.MouseButtonLeft, 1, 10)
	if strings.Contains(m.View(), "Help —") {
		t.Fatalf("expected a click to close the help, got:\n%s", m.View())
	}
}
