package tui_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
)

// newSnapshotFixture seeds one account, one category and one transaction so every screen has
// something concrete to render.
func newSnapshotFixture(t *testing.T) screentest.Fixture {
	t.Helper()
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{
		Name: "Checking", Kind: "checking", Currency: "EUR",
		OpeningBalance: "1250.00", OpenedOn: "2026-01-01",
	})
	fx.Category(t, app.AddCategoryRequest{Name: "Groceries"})
	fx.Tx(t, app.AddTxRequest{
		Account: "Checking", Amount: "-42.50", Description: "Supermarket",
		Category: "Groceries", Date: "2026-06-10",
	})
	return fx
}

// snapshot runs Root at 80x24 over the seeded fixture, types keys, waits for wait to appear in
// the rendered output, then diffs the ANSI-stripped final frame against
// testdata/<test name>.golden (refresh with `go test ./internal/tui -run <test> -update`).
func snapshot(t *testing.T, keys, wait string) {
	t.Helper()
	fx := newSnapshotFixture(t)
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	root := tui.New(uc, tui.Config{Locale: domain.LocaleENUS})

	tm := teatest.NewTestModel(t, root, teatest.WithInitialTermSize(80, 24))
	if keys != "" {
		tm.Type(keys)
	}

	// WaitFor reads (and so drains) tm.Output(); tee every byte it reads into captured so the
	// assertion below still has the full stream once the wait condition is satisfied.
	var captured bytes.Buffer
	teatest.WaitFor(t, io.TeeReader(tm.Output(), &captured), func(b []byte) bool {
		return bytes.Contains(b, []byte(wait))
	}, teatest.WithDuration(3*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	rest, err := io.ReadAll(tm.Output())
	if err != nil {
		t.Fatalf("read remaining output: %v", err)
	}
	captured.Write(rest)

	// Compare only the rendered text, not the raw escape stream: cursor-movement/clear
	// trailers that the terminal libraries emit on quit vary across patch versions of
	// bubbletea/colorprofile and are not part of what the screen actually renders, so
	// pinning to them would make the goldens brittle to unrelated dependency bumps.
	golden.RequireEqual(t, []byte(ansi.Strip(captured.String())))
}

func TestDashboardSnapshot(t *testing.T) {
	snapshot(t, "", "Checking")
}

func TestTransactionsSnapshot(t *testing.T) {
	snapshot(t, "2", "Supermarket")
}

func TestAccountsSnapshot(t *testing.T) {
	snapshot(t, "3", "Checking")
}
