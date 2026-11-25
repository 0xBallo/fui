package tui_test

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens"
)

// forceColorProfile pins lipgloss's default renderer to profile for the duration of the test,
// restoring the prior value on cleanup. Styles are built with lipgloss.NewStyle() against that
// shared default renderer, and Renderer.ColorProfile() caches its first result for the whole
// process (via sync.Once) rather than re-reading NO_COLOR/termenv on every render: once any test
// in this binary has triggered that cache, a plain t.Setenv("NO_COLOR", "1") would have no
// effect. Pinning the profile directly keeps every theme snapshot test below deterministic
// regardless of run order.
func forceColorProfile(t *testing.T, profile termenv.Profile) {
	t.Helper()
	r := lipgloss.DefaultRenderer()
	prev := r.ColorProfile()
	r.SetColorProfile(profile)
	t.Cleanup(func() { r.SetColorProfile(prev) })
}

// themeSnapshot renders Root with the given theme at 80x24 (FUI's minimum terminal size) and the
// color profile pinned by forceColorProfile's caller, types keys, and diffs the raw (not
// ANSI-stripped: that would hide the whole point, the tokens the theme chose) final view, once
// wait has appeared, against testdata/<test name>.golden.
func themeSnapshot(t *testing.T, theme, keys, wait string) {
	t.Helper()
	fx := newSnapshotFixture(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	root := tui.New(uc, tui.Config{Locale: domain.LocaleENUS, Theme: theme})

	tm := teatest.NewTestModel(t, root, teatest.WithInitialTermSize(80, 24))
	if keys != "" {
		tm.Type(keys)
	}

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(wait))
	}, teatest.WithDuration(3*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	// Snapshot the final view, not the output stream: the renderer coalesces frames on a timer,
	// so which intermediate frames reach the stream varies run to run.
	final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	golden.RequireEqual(t, []byte(final.View()))
}

func TestDashboardSnapshotLatte(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "latte", "", "net of loans")
}

func TestDashboardSnapshotMocha(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "mocha", "", "net of loans")
}

func TestDashboardSnapshotTerminal(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "terminal", "", "net of loans")
}

func TestDashboardSnapshotHighContrast(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "high-contrast", "", "net of loans")
}

func TestDashboardSnapshotNoColor(t *testing.T) {
	forceColorProfile(t, termenv.Ascii)
	themeSnapshot(t, "mocha", "", "net of loans")
}

func TestTransactionsSnapshotLatte(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "latte", "2", "booked")
}

func TestTransactionsSnapshotMocha(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "mocha", "2", "booked")
}

func TestTransactionsSnapshotTerminal(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "terminal", "2", "booked")
}

func TestTransactionsSnapshotHighContrast(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "high-contrast", "2", "booked")
}

func TestTransactionsSnapshotNoColor(t *testing.T) {
	forceColorProfile(t, termenv.Ascii)
	themeSnapshot(t, "mocha", "2", "booked")
}

// filteredTransactionsSnapshot opens Transactions through a drill-down SwitchScreenMsg at 80x24
// and diffs the final view against testdata/<test name>.golden.
func filteredTransactionsSnapshot(t *testing.T, theme string) {
	t.Helper()
	fx := newSnapshotFixture(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	var m tea.Model = tui.New(uc, tui.Config{Locale: domain.LocaleENUS, Theme: theme})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := m.Update(screens.SwitchScreenMsg{
		Target: screens.TabTransactions,
		Filter: &app.ListTxRequest{Categories: []string{"Groceries"}, From: "2026-06-01", To: "2026-06-30"},
	})
	m = run(t, m, cmd)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestFilteredTransactionsSnapshotLatte(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	filteredTransactionsSnapshot(t, "latte")
}

func TestFilteredTransactionsSnapshotMocha(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	filteredTransactionsSnapshot(t, "mocha")
}

func TestFilteredTransactionsSnapshotTerminal(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	filteredTransactionsSnapshot(t, "terminal")
}

func TestFilteredTransactionsSnapshotHighContrast(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	filteredTransactionsSnapshot(t, "high-contrast")
}

func TestFilteredTransactionsSnapshotNoColor(t *testing.T) {
	forceColorProfile(t, termenv.Ascii)
	filteredTransactionsSnapshot(t, "mocha")
}
