package report_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	scr "github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/report"
	"github.com/0xballo/fui/internal/tui/theme"
)

type compareNoDetector struct{}

func (compareNoDetector) Dark(time.Duration) (dark, ok bool) { return true, true }

func compareScreen(t *testing.T, th theme.Theme) scr.Screen {
	t.Helper()
	h := newHarness(t)
	h.fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Groceries", Kind: "expense"})
	h.fx.Category(t, app.AddCategoryRequest{Name: "Transport", Kind: "expense"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-100", Description: "Shop", Category: "Groceries", Date: "2026-05-10"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-150", Description: "Shop", Category: "Groceries", Date: "2026-06-10"})
	h.fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-40", Description: "Bus", Category: "Transport", Date: "2026-06-11"})
	m := report.New(h.uc, domain.LocaleENUS, 1, th)
	m.SetCompare(h.uc.analytics)
	m.SetSize(80, 24)
	return run(t, m, m.Init())
}

func TestCompareHiddenWithoutUseCases(t *testing.T) {
	h := newHarness(t)
	s := newFixture(t, h)
	s = run(t, s, s.Init())
	s, cmd := sendRunes(s, "c")
	s = run(t, s, cmd)
	if strings.Contains(s.View(), "Compare periods") || strings.Contains(s.View(), "c compare") {
		t.Fatalf("compare must be hidden:\n%s", s.View())
	}
}

// offers reports whether s exposes a binding with help key k, which Root shows in its hint line.
func offers(s scr.Screen, k string) bool {
	for _, b := range s.KeyMap() {
		if b.Help().Key == k {
			return true
		}
	}
	return false
}

func TestCompareViewAndToggle(t *testing.T) {
	s := compareScreen(t, theme.Default())
	if !offers(s, "c") {
		t.Fatalf("key map should offer compare: %v", s.KeyMap())
	}
	s, cmd := sendRunes(s, "c")
	s = run(t, s, cmd)
	v := s.View()
	for _, want := range []string{"Compare periods", "Groceries", "+50.0%", "n/a", "TOTAL", "+90.00"} {
		if !strings.Contains(v, want) {
			t.Fatalf("missing %q in:\n%s", want, v)
		}
	}
	s, cmd = sendRunes(s, "t")
	s = run(t, s, cmd)
	if v := s.View(); strings.Contains(v, "Groceries") || !strings.Contains(v, "TOTAL") {
		t.Fatalf("by-category off should show only the total:\n%s", v)
	}
	s, _ = sendKey(s, tea.KeyEsc)
	if !strings.Contains(s.View(), "Net worth") {
		t.Fatalf("esc should return to the report:\n%s", s.View())
	}
}

func TestComparePeriodsForm(t *testing.T) {
	s := compareScreen(t, theme.Default())
	s, cmd := sendRunes(s, "c")
	s = run(t, s, cmd)
	s, _ = sendRunes(s, "p")
	if !s.InputActive() {
		t.Fatal("form should capture input")
	}
	s, _ = sendRunes(s, "last-year")
	s, _ = sendKey(s, tea.KeyEnter) // accept the suggestion
	s, _ = sendKey(s, tea.KeyEnter) // next field
	s, _ = sendRunes(s, "this-year")
	s, _ = sendKey(s, tea.KeyEnter) // accept the suggestion
	s, cmd = sendKey(s, tea.KeyEnter)
	s = run(t, s, cmd)
	if s.InputActive() {
		t.Fatal("form should close on submit")
	}
	if v := s.View(); !strings.Contains(v, "2025-01-01") || !strings.Contains(v, "2026-12-31") {
		t.Fatalf("periods not applied:\n%s", v)
	}
}

func TestCompareThemesAndNoColor(t *testing.T) {
	var base string
	for _, name := range []string{"latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"} {
		s := compareScreen(t, theme.Resolve(theme.Config{Theme: name}, compareNoDetector{}))
		s, cmd := sendRunes(s, "c")
		s = run(t, s, cmd)
		plain := ansi.Strip(s.View())
		if base == "" {
			base = plain
		} else if plain != base {
			t.Fatalf("theme %s changes the text:\n%s\n---\n%s", name, plain, base)
		}
	}
	golden.RequireEqual(t, []byte(base))
}

type teaScreen struct{ scr scr.Screen }

func (m *teaScreen) Init() tea.Cmd { return m.scr.Init() }

func (m *teaScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.scr.Update(msg)
	m.scr = next
	return m, cmd
}

func (m *teaScreen) View() tea.View { return tea.NewView(m.scr.View()) }

func TestCompareTeatest(t *testing.T) {
	s := compareScreen(t, theme.Default())
	tm := teatest.NewTestModel(t, &teaScreen{scr: s}, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("710.00")) },
		teatest.WithDuration(15*time.Second))
	tm.Type("c")
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("TOTAL")) },
		teatest.WithDuration(15*time.Second))
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
}
