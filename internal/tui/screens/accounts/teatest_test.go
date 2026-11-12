package accounts_test

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/accounts"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type screenModel struct {
	scr           screens.Screen
	width, height int
}

func (m *screenModel) Init() tea.Cmd {
	m.scr.SetSize(m.width, m.height)
	return m.scr.Init()
}

func (m *screenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.scr.SetSize(size.Width, size.Height)
	}
	next, cmd := m.scr.Update(msg)
	m.scr = next
	return m, cmd
}

func (m *screenModel) View() string { return m.scr.View() }

func snapshot(t *testing.T, model tea.Model, ready, wait string, act func(*teatest.TestModel)) {
	t.Helper()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(ready))
	}, teatest.WithDuration(3*time.Second))
	if act != nil {
		act(tm)
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
	golden.RequireEqual(t, []byte(ansi.Strip(final.View())))
}

func TestAccountsReconcileSnapshot(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-10", Description: "Store", Date: "2026-06-01", Status: "booked"})

	scr := screens.Screen(accounts.New(fx.Service, domain.LocaleENUS, theme.Default()))
	model := &screenModel{scr: scr, width: 100, height: 30}
	snapshot(t, model, "Checking", "Reconcile preview", func(tm *teatest.TestModel) {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	})
}
