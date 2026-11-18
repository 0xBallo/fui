package components_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

func bind(k, desc string) key.Binding { return key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc)) }

func helpFixtures() (global, screen []key.Binding) {
	off := bind("z", "hidden")
	off.SetEnabled(false)
	global = []key.Binding{bind("?", "help"), bind("q", "quit"), off, key.NewBinding(key.WithKeys("x"))}
	screen = []key.Binding{bind("up", "move"), bind("n", "new"), bind("e", "edit"), bind("d", "delete"), bind("/", "filter")}
	return global, screen
}

func TestHelpBarCollapsed(t *testing.T) {
	g, s := helpFixtures()
	for _, w := range []int{40, 80, 200} {
		hb := components.NewHelpBar(theme.Default())
		hb.SetWidth(w)
		out := ansi.Strip(hb.View(g, s, "Screen", 10))
		if lipgloss.Height(out) != 1 || lipgloss.Width(out) > w {
			t.Fatalf("w=%d: want one line within width, got %q", w, out)
		}
		if !strings.HasPrefix(out, "n new") || strings.Contains(out, "move") || strings.Contains(out, "hidden") {
			t.Fatalf("w=%d: bad content %q", w, out)
		}
		if w == 200 && !strings.HasSuffix(out, "q quit") {
			t.Fatalf("w=200: global must follow screen, got %q", out)
		}
		if w == 80 {
			golden.RequireEqual(t, []byte(out))
		}
	}
	hb := components.NewHelpBar(theme.Default())
	hb.SetWidth(20)
	if out := ansi.Strip(hb.View(g, s, "Screen", 10)); !strings.HasSuffix(out, "…") {
		t.Fatalf("want ellipsis, got %q", out)
	}
}

func TestHelpBarExpanded(t *testing.T) {
	g, s := helpFixtures()
	hb := components.NewHelpBar(theme.Default())
	hb.SetWidth(80)
	hb.Toggle()
	if !hb.Expanded() {
		t.Fatal("want expanded")
	}
	out := ansi.Strip(hb.View(g, s, "Screen", 20))
	if !strings.Contains(out, "Global") || !strings.Contains(out, "Screen") || strings.Contains(out, "hidden") {
		t.Fatalf("bad expanded view %q", out)
	}
	golden.RequireEqual(t, []byte(out))

	wrapped := ansi.Strip(hb.View(g, s, "Screen", 5))
	if h := lipgloss.Height(wrapped); h > 5 {
		t.Fatalf("height %d exceeds 5:\n%s", h, wrapped)
	}
	if !strings.Contains(wrapped, "help") || !strings.Contains(wrapped, "filter") {
		t.Fatalf("entries lost:\n%s", wrapped)
	}

	hb.Toggle()
	if hb.Expanded() {
		t.Fatal("want collapsed")
	}
}
