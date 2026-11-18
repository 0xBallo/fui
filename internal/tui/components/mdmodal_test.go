package components_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func mdTheme(t *testing.T, name string) theme.Theme {
	t.Helper()
	return theme.Resolve(theme.Config{Theme: name}, nil)
}

func longDoc() string {
	var b strings.Builder
	b.WriteString("# Title\n\n")
	for i := range 40 {
		b.WriteString("- item ")
		b.WriteString(string(rune('A' + i%26)))
		b.WriteString("\n")
	}
	return b.String()
}

func key(m components.MarkdownModal, k tea.KeyType) components.MarkdownModal {
	m, _ = m.Update(tea.KeyMsg{Type: k})
	return m
}

func TestMarkdownModalRenders(t *testing.T) {
	m := components.NewMarkdownModal(mdTheme(t, "mocha"), "# Heading\n\n- first\n- second\n")
	if m.View() != "" {
		t.Fatal("closed modal must render nothing")
	}
	m.Open()
	m.SetSize(60, 20)
	v := ansi.Strip(m.View())
	for _, want := range []string{"Heading", "first", "second", "esc close"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestMarkdownModalScrollAndClose(t *testing.T) {
	m := components.NewMarkdownModal(mdTheme(t, "mocha"), longDoc())
	m.SetSize(60, 14)
	m.Open()
	top := m.View()
	m = key(m, tea.KeyPgDown)
	if m.View() == top {
		t.Error("pgdown did not scroll")
	}
	m = key(m, tea.KeyHome)
	if m.View() != top {
		t.Error("home did not return to top")
	}
	m = key(m, tea.KeyEnd)
	if m.View() == top {
		t.Error("end did not scroll")
	}
	m = key(m, tea.KeyEsc)
	if m.IsOpen() {
		t.Error("esc did not close")
	}
	m.Open()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.IsOpen() {
		t.Error("i did not close")
	}
}

func TestMarkdownModalNoColorGolden(t *testing.T) {
	m := components.NewMarkdownModal(mdTheme(t, "terminal"), "# Heading\n\nSome **bold** text.\n\n- one\n- two\n")
	m.SetSize(50, 16)
	m.Open()
	golden.RequireEqual(t, []byte(ansi.Strip(m.View())))
}

func TestMarkdownModalAsciiProfileHasNoEscapes(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := components.NewMarkdownModal(mdTheme(t, "mocha"), "# Heading\n\nSome **bold** `code`\n\n- one\n")
	m.SetSize(50, 16)
	m.Open()
	if v := m.View(); strings.Contains(v, "\x1b") {
		t.Errorf("view contains ANSI escapes under Ascii profile:\n%q", v)
	}
}
