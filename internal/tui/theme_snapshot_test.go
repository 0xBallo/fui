package tui_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
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
// ANSI-stripped: that would hide the whole point, the tokens the theme chose) output captured up
// to the point wait appears against testdata/<test name>.golden. Capturing only up to wait
// rather than Quit's full output keeps goldens small and avoids the quit-trailer brittleness
// that snapshot (in teatest_test.go) strips ANSI to dodge.
func themeSnapshot(t *testing.T, theme, keys, wait string) {
	t.Helper()
	fx := newSnapshotFixture(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	root := tui.New(uc, tui.Config{Locale: domain.LocaleENUS, Theme: theme})

	tm := teatest.NewTestModel(t, root, teatest.WithInitialTermSize(80, 24))
	if keys != "" {
		tm.Type(keys)
	}

	var captured bytes.Buffer
	teatest.WaitFor(t, io.TeeReader(tm.Output(), &captured), func(b []byte) bool {
		return bytes.Contains(b, []byte(wait))
	}, teatest.WithDuration(3*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	// Drain the remaining (quit-trailer) output so the program can finish; it's intentionally
	// excluded from the golden.
	if _, err := io.ReadAll(tm.Output()); err != nil {
		t.Fatalf("read remaining output: %v", err)
	}

	golden.RequireEqual(t, captured.Bytes())
}

func TestDashboardSnapshotLatte(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "latte", "", "Checking")
}

func TestDashboardSnapshotMocha(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "mocha", "", "Checking")
}

func TestDashboardSnapshotTerminal(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "terminal", "", "Checking")
}

func TestDashboardSnapshotHighContrast(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "high-contrast", "", "Checking")
}

func TestDashboardSnapshotNoColor(t *testing.T) {
	forceColorProfile(t, termenv.Ascii)
	themeSnapshot(t, "mocha", "", "Checking")
}

func TestTransactionsSnapshotLatte(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "latte", "2", "Supermarket")
}

func TestTransactionsSnapshotMocha(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "mocha", "2", "Supermarket")
}

func TestTransactionsSnapshotTerminal(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "terminal", "2", "Supermarket")
}

func TestTransactionsSnapshotHighContrast(t *testing.T) {
	forceColorProfile(t, termenv.TrueColor)
	themeSnapshot(t, "high-contrast", "2", "Supermarket")
}

func TestTransactionsSnapshotNoColor(t *testing.T) {
	forceColorProfile(t, termenv.Ascii)
	themeSnapshot(t, "mocha", "2", "Supermarket")
}
