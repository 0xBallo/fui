package budget_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens/budget"
)

func TestBudgetHelpModal(t *testing.T) {
	h := newHarness(t, domain.BudgetCap)
	s := newFixture(t, h)
	s = run(t, s, s.Init())
	s, _ = sendRunes(s, "i")
	if !s.InputActive() {
		t.Fatal("expected InputActive true while the help modal is open")
	}
	view := s.View()
	for _, want := range []string{"Caps", "Envelopes"} {
		if !strings.Contains(view, want) {
			t.Errorf("help modal missing %q:\n%s", want, view)
		}
	}
	s, _ = sendRunes(s, "s")
	if !s.InputActive() || strings.Contains(s.View(), "Set amount") {
		t.Fatal("keys must not reach the screen while the modal is open")
	}
	s, _ = sendKey(s, tea.KeyEsc)
	if s.InputActive() {
		t.Fatal("expected InputActive false after esc")
	}
}

func TestBudgetKeyMapNoCollisions(t *testing.T) {
	seen := map[string]string{}
	for _, b := range budget.DefaultKeyMap().Bindings() {
		for _, k := range b.Keys() {
			if prev, ok := seen[k]; ok {
				t.Errorf("key %q bound twice (%s, %s)", k, prev, b.Help().Desc)
			}
			seen[k] = b.Help().Desc
		}
	}
	for _, g := range []string{"1", "2", "3", "4", "5", "6", "7", "?", "q"} {
		if _, ok := seen[g]; ok {
			t.Errorf("key %q collides with a global key", g)
		}
	}
	if _, ok := seen["i"]; !ok {
		t.Error("explain key i missing")
	}
}
