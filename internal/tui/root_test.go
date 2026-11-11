package tui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
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
