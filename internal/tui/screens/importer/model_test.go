package importer_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/importer"
	"github.com/0xballo/fui/internal/tui/theme"
)

// run executes cmd and any cmd that Update returns in reaction, until it settles, returning the
// resulting screen. Every tea.Cmd in this screen finishes all its I/O and returns one terminal
// message, so draining one chain is enough.
func run(t *testing.T, scr screens.Screen, cmd tea.Cmd) screens.Screen {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		scr, cmd = scr.Update(msg)
	}
	return scr
}

func sendKey(scr screens.Screen, k tea.KeyType) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyMsg{Type: k})
}

func sendRunes(scr screens.Screen, s string) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func newScreen(t *testing.T) (fixture, screens.Screen) {
	t.Helper()
	fx := newFixture()
	scr := screens.Screen(importer.New(fx, domain.LocaleENUS, theme.Default()))
	scr.SetSize(100, 30)
	return fx, scr
}

// fillSourceForm types connector, file, profile, account and since into the source form in
// order, pressing enter between fields, and returns the command the last enter produced. The
// connector field has suggestions (the discovered connectors), so an exact match opens a
// dropdown that the first enter accepts instead of advancing: a second enter is needed then.
func fillSourceForm(scr screens.Screen, connector, file, profile, account, since string) (screens.Screen, tea.Cmd) {
	var cmd tea.Cmd
	for i, v := range []string{connector, file, profile, account, since} {
		if v != "" {
			scr, _ = sendRunes(scr, v)
		}
		scr, cmd = sendKey(scr, tea.KeyEnter)
		if i == 0 && v != "" {
			scr, cmd = sendKey(scr, tea.KeyEnter)
		}
	}
	return scr, cmd
}

func TestImportFUIPlansAndApplies(t *testing.T) {
	fx, scr := newScreen(t)
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-1",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
	)
	scr = run(t, scr, scr.Init())

	scr, cmd := fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Map unmapped accounts") {
		t.Fatalf("expected the mapping form for the unmapped ref, got:\n%s", view)
	}

	scr, _ = sendRunes(scr, "create")
	scr, _ = sendKey(scr, tea.KeyEnter)   // accept the "+ create new account" suggestion
	scr, cmd = sendKey(scr, tea.KeyEnter) // submit the mapping form
	scr = run(t, scr, cmd)

	view = scr.View()
	if !strings.Contains(view, "Import preview") || !strings.Contains(view, "Pizza") {
		t.Fatalf("expected the row preview with Pizza, got:\n%s", view)
	}
	if !strings.Contains(view, "new=1") {
		t.Fatalf("expected one new row, got:\n%s", view)
	}

	scr, cmd = sendKey(scr, tea.KeyEnter)
	if cmd != nil {
		t.Fatalf("expected the confirm dialog to need its own keypress, not an immediate command")
	}
	if !strings.Contains(scr.View(), "Confirm import") {
		t.Fatalf("expected the confirm dialog, got:\n%s", scr.View())
	}

	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)

	view = scr.View()
	if !strings.Contains(view, "Import complete") || !strings.Contains(view, "transactions=1") {
		t.Fatalf("expected the result summary with transactions=1, got:\n%s", view)
	}

	accts, err := fx.ListAccounts(context.Background(), app.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accts) != 1 || accts[0].Account.Name != "Checking" {
		t.Fatalf("expected the Checking account created by the import, got %+v", accts)
	}
}

func TestImportMapsUnmappedRefToExistingAccount(t *testing.T) {
	fx, scr := newScreen(t)
	plan, err := fx.PlanAddAccount(context.Background(), app.AddAccountRequest{
		Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount: %v", err)
	}
	if _, err := fx.ApplyAddAccount(context.Background(), plan); err != nil {
		t.Fatalf("ApplyAddAccount: %v", err)
	}
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-2",
		fuiAccount("acc-1", "Current account", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -500, "Coffee"),
	)
	scr = run(t, scr, scr.Init())

	scr, cmd := fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "Map unmapped accounts") {
		t.Fatalf("expected the mapping form, got:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "Checking")
	scr, _ = sendKey(scr, tea.KeyEnter) // accept the "Checking" suggestion
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Import preview") || !strings.Contains(view, "Coffee") {
		t.Fatalf("expected the row preview with Coffee, got:\n%s", view)
	}

	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd = sendRunes(scr, "y")
	run(t, scr, cmd)

	accts, err := fx.ListAccounts(context.Background(), app.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accts) != 1 {
		t.Fatalf("expected the mapping to bind to the existing account instead of creating one, got %+v", accts)
	}
}

func TestImportToggleIncludeExcludesRowFromApply(t *testing.T) {
	fx, scr := newScreen(t)
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-3",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
	)
	scr = run(t, scr, scr.Init())

	scr, cmd := fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "create")
	scr, _ = sendKey(scr, tea.KeyEnter)   // accept the "+ create new account" suggestion
	scr, cmd = sendKey(scr, tea.KeyEnter) // submit the mapping form
	scr = run(t, scr, cmd)

	scr, _ = sendRunes(scr, " ") // toggle the single row off
	if !strings.Contains(scr.View(), "included=0") {
		t.Fatalf("expected included=0 after toggling off, got:\n%s", scr.View())
	}

	scr, cmd = sendKey(scr, tea.KeyEnter)
	if !strings.Contains(scr.View(), "nothing to import") {
		t.Fatalf("expected a notice refusing to confirm with no included rows, got:\n%s", scr.View())
	}
	_ = cmd
}

func TestImportBackAndCancelKeysReturnToEarlierModes(t *testing.T) {
	fx, scr := newScreen(t)
	if scr.Title() != "Import" {
		t.Fatalf("Title() = %q, want Import", scr.Title())
	}
	if len(scr.KeyMap()) == 0 {
		t.Fatal("KeyMap() returned no bindings")
	}
	scr.SetSize(100, 30)
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-4",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
	)
	scr = run(t, scr, scr.Init())
	if !scr.InputActive() {
		t.Fatal("InputActive() should be true on the source form")
	}

	scr, cmd := fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)

	// esc on the mapping form abandons the import and returns to the source form.
	scr, _ = sendKey(scr, tea.KeyEsc)
	if !strings.Contains(scr.View(), "Import") || strings.Contains(scr.View(), "Map unmapped accounts") {
		t.Fatalf("expected esc to abandon the mapping form, got:\n%s", scr.View())
	}

	scr, cmd = fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "create")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	// esc on the preview also resets to the source form.
	scr, _ = sendKey(scr, tea.KeyEsc)
	if !strings.Contains(scr.View(), "enter next/plan") {
		t.Fatalf("expected esc on the preview to reset to the source form, got:\n%s", scr.View())
	}

	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-5",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
	)
	scr, cmd = fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "create")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	scr, cmd = sendKey(scr, tea.KeyEnter) // open the confirm dialog
	// "n" cancels the confirm dialog and returns to the preview.
	scr, _ = sendRunes(scr, "n")
	if !strings.Contains(scr.View(), "Import preview") {
		t.Fatalf("expected cancelling the confirm dialog to return to the preview, got:\n%s", scr.View())
	}
	_ = cmd

	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Import complete") {
		t.Fatalf("expected the result screen, got:\n%s", scr.View())
	}
	// "n" on the result screen starts a new import, back on the source form.
	scr, _ = sendRunes(scr, "n")
	if !strings.Contains(scr.View(), "enter next/plan") {
		t.Fatalf("expected \"n\" on the result screen to reset to the source form, got:\n%s", scr.View())
	}
}

func TestImportDuplicateRowIsFlagged(t *testing.T) {
	fx, scr := newScreen(t)
	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-6",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
	)
	scr = run(t, scr, scr.Init())
	scr, cmd := fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "create")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd = sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "n") // start a new import of the same statement

	fx.connectors.fixtures["fui"] = fuiStream("fui", "statement.fui.jsonl", "sha-6",
		fuiAccount("acc-1", "Checking", "EUR"),
		fuiTx("acc-1", "tx-1", "2026-06-01", -1000, "Pizza"),
	)
	scr, cmd = fillSourceForm(scr, "fui", "statement.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "= dup") || !strings.Contains(view, "already imported as batch") {
		t.Fatalf("expected the replayed row to be flagged as a duplicate, got:\n%s", view)
	}
}

func TestImportUnknownConnectorKeepsSourceForm(t *testing.T) {
	_, scr := newScreen(t)
	scr = run(t, scr, scr.Init())

	// "unknown" matches no connector suggestion, so a single enter per field advances normally.
	scr, _ = sendRunes(scr, "unknown")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, _ = sendRunes(scr, "statement.csv")
	var cmd tea.Cmd
	for i := 0; i < 4; i++ {
		scr, cmd = sendKey(scr, tea.KeyEnter)
	}
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "enter next/plan") {
		t.Fatalf("expected a connector error to keep the user on the source form, got:\n%s", scr.View())
	}
}

func TestImportMissingFileShowsNotice(t *testing.T) {
	_, scr := newScreen(t)
	scr = run(t, scr, scr.Init())

	scr, cmd := fillSourceForm(scr, "fui", "", "", "", "")
	scr = run(t, scr, cmd)

	if !strings.Contains(scr.View(), "File is required") {
		t.Fatalf("expected a notice about the missing file, got:\n%s", scr.View())
	}
}
