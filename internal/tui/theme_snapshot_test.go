package tui_test

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
)

// plainView is the ANSI-stripped frame of m: the text assertions compare it, as they did when
// lipgloss v1 rendered without styles outside a terminal.
func plainView(m tea.Model) string { return ansi.Strip(m.View().Content) }

// themeSnapshot renders Root with the given theme at 80x24 (FUI's minimum terminal size), types keys, and diffs the final view converted to
// profile p (not ANSI-stripped: that would hide the whole point, the tokens the theme chose) final view, once
// wait has appeared, against testdata/<test name>.golden.
func themeSnapshot(t *testing.T, theme, keys, wait string, p colorprofile.Profile) {
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
	golden.RequireEqual(t, []byte(screentest.Downsample(final.View().Content, p)))
}

func TestDashboardSnapshotLatte(t *testing.T) {
	themeSnapshot(t, "latte", "", "net of loans", colorprofile.TrueColor)
}

func TestDashboardSnapshotMocha(t *testing.T) {
	themeSnapshot(t, "mocha", "", "net of loans", colorprofile.TrueColor)
}

func TestDashboardSnapshotTerminal(t *testing.T) {
	themeSnapshot(t, "terminal", "", "net of loans", colorprofile.TrueColor)
}

func TestDashboardSnapshotHighContrast(t *testing.T) {
	themeSnapshot(t, "high-contrast", "", "net of loans", colorprofile.TrueColor)
}

func TestDashboardSnapshotNoColor(t *testing.T) {
	themeSnapshot(t, "mocha", "", "net of loans", colorprofile.Ascii)
}

func TestTransactionsSnapshotLatte(t *testing.T) {
	themeSnapshot(t, "latte", "2", "booked", colorprofile.TrueColor)
}

func TestTransactionsSnapshotMocha(t *testing.T) {
	themeSnapshot(t, "mocha", "2", "booked", colorprofile.TrueColor)
}

func TestTransactionsSnapshotTerminal(t *testing.T) {
	themeSnapshot(t, "terminal", "2", "booked", colorprofile.TrueColor)
}

func TestTransactionsSnapshotHighContrast(t *testing.T) {
	themeSnapshot(t, "high-contrast", "2", "booked", colorprofile.TrueColor)
}

func TestTransactionsSnapshotNoColor(t *testing.T) {
	themeSnapshot(t, "mocha", "2", "booked", colorprofile.Ascii)
}

// filteredTransactionsSnapshot opens Transactions through a drill-down SwitchScreenMsg at 80x24
// and diffs the final view against testdata/<test name>.golden.
func filteredTransactionsSnapshot(t *testing.T, theme string, p colorprofile.Profile) {
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
	golden.RequireEqual(t, []byte(screentest.Downsample(m.View().Content, p)))
}

func TestFilteredTransactionsSnapshotLatte(t *testing.T) {
	filteredTransactionsSnapshot(t, "latte", colorprofile.TrueColor)
}

func TestFilteredTransactionsSnapshotMocha(t *testing.T) {
	filteredTransactionsSnapshot(t, "mocha", colorprofile.TrueColor)
}

func TestFilteredTransactionsSnapshotTerminal(t *testing.T) {
	filteredTransactionsSnapshot(t, "terminal", colorprofile.TrueColor)
}

func TestFilteredTransactionsSnapshotHighContrast(t *testing.T) {
	filteredTransactionsSnapshot(t, "high-contrast", colorprofile.TrueColor)
}

func TestFilteredTransactionsSnapshotNoColor(t *testing.T) {
	filteredTransactionsSnapshot(t, "mocha", colorprofile.Ascii)
}
