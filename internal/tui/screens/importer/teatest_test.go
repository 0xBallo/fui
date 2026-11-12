package importer_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/importer"
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

	var captured bytes.Buffer
	teatest.WaitFor(t, io.TeeReader(tm.Output(), &captured), func(b []byte) bool {
		return bytes.Contains(b, []byte("n new import"))
	}, teatest.WithDuration(3*time.Second))
	typeRunes(tm, "n") // open the source form from the idle page
	teatest.WaitFor(t, io.TeeReader(tm.Output(), &captured), func(b []byte) bool {
		return bytes.Contains(b, []byte(ready))
	}, teatest.WithDuration(3*time.Second))
	if act != nil {
		act(tm)
	}
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
	golden.RequireEqual(t, []byte(ansi.Strip(captured.String())))
}

func typeRunes(tm *teatest.TestModel, s string) {
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func TestImportPreviewSnapshot(t *testing.T) {
	fx := newFixture()
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-snap",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
		fuiTx("acc-1", "tx-2", "2026-06-02", 50000, "Salary"),
	)

	scr := screens.Screen(importer.New(fx, domain.LocaleENUS, theme.Default()))
	model := &screenModel{scr: scr, width: 100, height: 30}
	snapshot(t, model, "> csv", "Map unmapped accounts", func(tm *teatest.TestModel) {
		typeRunes(tm, "fui")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // accept the "fui" suggestion
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // move to the file field
		typeRunes(tm, "statement.fui.jsonl")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // move to the CSV profile field
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // move to the account ref field
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // move to the since field
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // submit, planning the import
	})
}

func TestImportConfirmDialogSnapshot(t *testing.T) {
	fx := newFixture()
	// The connector ref matches an existing account's name exactly, so the plan auto-binds it
	// (see builder.resolve in internal/app/importing/build.go) with no mapping round trip,
	// keeping this snapshot to the same single async step as TestImportPreviewSnapshot.
	plan, err := fx.PlanAddAccount(context.Background(), app.AddAccountRequest{
		Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount: %v", err)
	}
	if _, err := fx.ApplyAddAccount(context.Background(), plan); err != nil {
		t.Fatalf("ApplyAddAccount: %v", err)
	}
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-snap2",
		fuiAccount("Checking", "Checking", "EUR"),
		fuiTx("Checking", "tx-1", "2026-06-01", -1000, "Pizza"),
	)

	scr := screens.Screen(importer.New(fx, domain.LocaleENUS, theme.Default()))
	model := &screenModel{scr: scr, width: 100, height: 30}
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))

	var captured bytes.Buffer
	waitForText := func(text string) {
		t.Helper()
		teatest.WaitFor(t, io.TeeReader(tm.Output(), &captured), func(b []byte) bool {
			return bytes.Contains(b, []byte(text))
		}, teatest.WithDuration(3*time.Second))
	}

	// Wait for the connector field's suggestions to load (the "> csv" dropdown line) before
	// typing, so the "fui" keystroke race never lands before connectorsLoadedMsg: otherwise the
	// dropdown-accept double-enter below and the resulting frame sequence are non-deterministic.
	waitForText("n new import")
	typeRunes(tm, "n") // open the source form from the idle page
	waitForText("> csv")
	typeRunes(tm, "fui")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // accept the "fui" suggestion
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // move to the file field
	typeRunes(tm, "statement.fui.jsonl")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // submit, planning the import onto the preview

	waitForText("Import preview")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // open the confirm dialog

	teatest.WaitFor(t, io.TeeReader(tm.Output(), &captured), func(b []byte) bool {
		return bytes.Contains(b, []byte("Confirm import"))
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
	golden.RequireEqual(t, []byte(ansi.Strip(captured.String())))
}
