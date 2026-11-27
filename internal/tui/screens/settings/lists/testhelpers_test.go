package lists_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/tui/screens"
)

// run executes cmd and any cmd that Update returns in reaction, until it settles, returning the
// resulting screen. Every tea.Cmd in these screens finishes all its I/O and returns one terminal
// message, so draining one chain is enough.
func run(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		scr, cmd = scr.Update(msg)
	}
	return scr
}

func sendKey(scr screens.Screen, k rune) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyPressMsg{Code: k})
}

func sendRunes(scr screens.Screen, s string) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyPressMsg{Code: []rune(s)[0], Text: s})
}
