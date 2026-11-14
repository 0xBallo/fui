package dashboard_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/dashboard"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type stubInsights struct {
	mu        sync.Mutex
	items     []reporting.InsightView
	dismissed []string
	accepted  []string
	runErr    error
	created   bool
}

func (s *stubInsights) Run(context.Context) ([]reporting.InsightView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.items), s.runErr
}

func (s *stubInsights) Dismiss(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dismissed = append(s.dismissed, key)
	s.items = slices.DeleteFunc(s.items, func(v reporting.InsightView) bool { return v.Key == key })
	return nil
}

func (s *stubInsights) AcceptSubscription(_ context.Context, key string) (reporting.AcceptResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accepted = append(s.accepted, key)
	s.items = slices.DeleteFunc(s.items, func(v reporting.InsightView) bool { return v.Key == key })
	return reporting.AcceptResult{Created: s.created}, nil
}

func sampleInsights() []reporting.InsightView {
	return []reporting.InsightView{
		{Kind: "anomaly", Key: "anomaly:grocer", Label: "Grocer", Detail: "3.0x the usual spend", Amount: reporting.Amount{Minor: 9000, Currency: "EUR"}},
		{Kind: "subscription", Key: "sub:netflix", Label: "Netflix", Detail: "monthly", Amount: reporting.Amount{Minor: 1299, Currency: "EUR"}},
		{Kind: "trend", Key: "trend:media", Label: "Media", Detail: "up 40% vs the last 3 months"},
	}
}

func insightScreen(t *testing.T, ins *stubInsights, th theme.Theme) screens.Screen {
	t.Helper()
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	m := dashboard.New(uc, domain.LocaleENUS, th, 1)
	m.SetInsights(ins)
	m.SetSize(80, 24)
	return runBatch(t, m, m.Init())
}

// runBatch is run for commands that may be tea.Batch: it expands every batched command.
func runBatch(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	if cmd == nil {
		return scr
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			scr = runBatch(t, scr, c)
		}
	default:
		next, c := scr.Update(msg)
		scr = runBatch(t, next, c)
	}
	return scr
}

func TestInsightPanelHiddenWithoutUseCases(t *testing.T) {
	_, scr := newFixture(t)
	scr = run(t, scr, scr.Init())
	if strings.Contains(scr.View(), "Insights") {
		t.Fatalf("panel must be hidden without InsightUseCases:\n%s", scr.View())
	}
}

func TestInsightPanelDismissAccept(t *testing.T) {
	ins := &stubInsights{items: sampleInsights(), created: true}
	scr := insightScreen(t, ins, theme.Default())
	view := scr.View()
	for _, want := range []string{"[anomaly] Grocer", "[subscription] Netflix", "[trend] Media", "90.00 EUR"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}

	// Unfocused, "a" still opens quick add instead of accepting.
	scr, _ = sendRunes(scr, "i")
	scr, cmd := sendRunes(scr, "x")
	scr = runBatch(t, scr, cmd)
	if !slices.Equal(ins.dismissed, []string{"anomaly:grocer"}) {
		t.Fatalf("dismissed = %v", ins.dismissed)
	}
	if strings.Contains(scr.View(), "Grocer") {
		t.Fatalf("dismissed insight still listed:\n%s", scr.View())
	}

	// Cursor is on the subscription now.
	scr, cmd = sendRunes(scr, "a")
	scr = runBatch(t, scr, cmd)
	if !slices.Equal(ins.accepted, []string{"sub:netflix"}) {
		t.Fatalf("accepted = %v", ins.accepted)
	}
	if strings.Contains(scr.View(), "Netflix") {
		t.Fatalf("accepted insight still listed:\n%s", scr.View())
	}
}

func TestInsightPanelAcceptRejectsNonSubscription(t *testing.T) {
	ins := &stubInsights{items: sampleInsights()}
	scr := insightScreen(t, ins, theme.Default())
	scr, _ = sendRunes(scr, "i")
	_, cmd := sendRunes(scr, "a")
	msg, ok := cmd().(screens.StatusMsg)
	if !ok || !strings.Contains(msg.Text, "subscription") || len(ins.accepted) != 0 {
		t.Fatalf("msg = %#v accepted = %v", cmd, ins.accepted)
	}
}

func TestInsightPanelFocusAndLeave(t *testing.T) {
	ins := &stubInsights{items: sampleInsights()}
	scr := insightScreen(t, ins, theme.Default())
	scr, _ = sendRunes(scr, "i")
	scr, _ = sendRunes(scr, "j")
	if v := scr.View(); !strings.Contains(v, "> [subscription] Netflix") {
		t.Fatalf("cursor not on second row:\n%s", v)
	}
	scr, _ = sendKey(scr, tea.KeyEsc)
	if v := scr.View(); strings.Contains(v, "> [") {
		t.Fatalf("still focused after esc:\n%s", v)
	}
	// With focus left, "a" opens quick add again.
	scr, _ = sendRunes(scr, "a")
	if !scr.InputActive() {
		t.Fatal("quick add should open when the panel is not focused")
	}
}

func TestInsightPanelModuleDisabledHides(t *testing.T) {
	ins := &stubInsights{runErr: app.ErrModuleDisabled}
	scr := insightScreen(t, ins, theme.Default())
	if strings.Contains(scr.View(), "Insights") {
		t.Fatalf("panel must hide when the module is disabled:\n%s", scr.View())
	}
}

type noDetector struct{}

func (noDetector) Dark(time.Duration) (dark, ok bool) { return true, true }

// TestInsightPanelThemesAndNoColor checks that the plain text is identical under every theme, so
// no information is carried by colour alone, and snapshots it at 80x24.
func TestInsightPanelThemesAndNoColor(t *testing.T) {
	var base string
	for _, name := range []string{"latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"} {
		th := theme.Resolve(theme.Config{Theme: name}, noDetector{})
		ins := &stubInsights{items: sampleInsights()}
		scr := insightScreen(t, ins, th)
		scr, _ = sendRunes(scr, "i")
		plain := ansi.Strip(scr.View())
		if base == "" {
			base = plain
		} else if plain != base {
			t.Fatalf("theme %s changes the text:\n%s\n---\n%s", name, plain, base)
		}
	}
	if lines := strings.Count(base, "\n") + 1; lines > 24 {
		t.Logf("dashboard with insights is %d lines tall", lines)
	}
	golden.RequireEqual(t, []byte(base))
}

type teaScreen struct{ scr screens.Screen }

func (m *teaScreen) Init() tea.Cmd { return m.scr.Init() }

func (m *teaScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.scr.Update(msg)
	m.scr = next
	return m, cmd
}

func (m *teaScreen) View() string { return m.scr.View() }

func TestInsightPanelTeatest(t *testing.T) {
	ins := &stubInsights{items: sampleInsights()}
	fx := screentest.New(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	d := dashboard.New(uc, domain.LocaleENUS, theme.Default(), 1)
	d.SetInsights(ins)
	d.SetSize(80, 24)
	tm := teatest.NewTestModel(t, &teaScreen{scr: d}, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("Netflix")) },
		teatest.WithDuration(3*time.Second))
	tm.Type("ix")
	teatest.WaitFor(t, tm.Output(), func([]byte) bool {
		ins.mu.Lock()
		defer ins.mu.Unlock()
		return len(ins.dismissed) > 0
	},
		teatest.WithDuration(3*time.Second))
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ins.dismissed, []string{"anomaly:grocer"}) {
		t.Fatalf("dismissed = %v", ins.dismissed)
	}
}
