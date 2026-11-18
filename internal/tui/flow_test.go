package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui"
)

func newFlowRoot(t *testing.T, width, height int) tea.Model {
	t.Helper()
	fx := newSnapshotFixture(t)
	cfg := routedConfig(module.Budget)
	root := tui.New(useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}, cfg)
	m, _ := root.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m
}

func TestDashboardToBudgetFlow(t *testing.T) {
	m := newFlowRoot(t, 100, 30)
	m = run(t, m, m.Init())

	before := m.View()
	m, cmd := sendRunes(m, "p")
	m = run(t, m, cmd)
	if m.View() == before {
		t.Fatalf("p did not change the dashboard period:\n%s", before)
	}

	m, cmd = sendRunes(m, "t")
	m = run(t, m, cmd)
	if !strings.Contains(m.View(), "Supermarket") {
		t.Fatalf("t did not open Transactions:\n%s", m.View())
	}

	m, _ = sendRunes(m, "?")
	if !strings.Contains(m.View(), "Global") {
		t.Fatalf("? did not expand the help:\n%s", m.View())
	}
	m, _ = sendRunes(m, "?")

	m, _ = sendRunes(m, "4")
	if !strings.Contains(m.View(), "Budget") {
		t.Fatalf("4 did not open Budget:\n%s", m.View())
	}
	m, _ = sendRunes(m, "i")
	open := m.View()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.View() == open {
		t.Fatalf("esc did not close the modal:\n%s", open)
	}
}

func TestEveryTabFitsMinimumSize(t *testing.T) {
	m := newFlowRoot(t, 80, 24)
	for _, k := range []string{"1", "2", "3", "4", "5", "6", "7"} {
		// Tabs 4-7 sit on stub ports: render them without running their load commands.
		m, _ = sendRunes(m, k)
		lines := strings.Split(m.View(), "\n")
		if len(lines) > 24 {
			t.Errorf("tab %s: %d rows, want <= 24", k, len(lines))
		}
		for _, l := range lines {
			if w := lipgloss.Width(l); w > 80 {
				t.Errorf("tab %s: line of width %d > 80: %q", k, w, l)
			}
		}
	}
}
