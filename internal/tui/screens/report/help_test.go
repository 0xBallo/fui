package report

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/tui/theme"
)

func TestHelpCoversEveryPredefinedKind(t *testing.T) {
	for _, k := range predefinedKinds() {
		if !strings.Contains(helpMarkdown, "**"+k.Label+"**") {
			t.Errorf("help.md does not explain %q", k.Label)
		}
	}
}

func TestHelpOpenClose(t *testing.T) {
	m := New(nil, "", 1, theme.Default())
	m.SetSize(100, 30)
	key := func(s string) { m.Update(tea.KeyPressMsg{Code: []rune(s)[0], Text: s}) }
	key("i")
	if !m.help.IsOpen() || !m.InputActive() || !strings.Contains(m.View(), "Net worth") {
		t.Fatalf("help should be open:\n%s", m.View())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.help.IsOpen() {
		t.Fatal("esc should close help")
	}
}

func TestKeyMapNoCollisions(t *testing.T) {
	seen := map[string]string{}
	for _, b := range DefaultKeyMap().Bindings() {
		for _, k := range b.Keys() {
			if k == "enter" {
				continue // run and run-saved share enter in different modes
			}
			if prev, ok := seen[k]; ok {
				t.Errorf("key %q bound twice (%s, %s)", k, prev, b.Help().Desc)
			}
			seen[k] = b.Help().Desc
		}
	}
}
