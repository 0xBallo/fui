package tui_test

import (
	"bytes"
	"strings"
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
	"github.com/0xballo/fui/internal/tui/screens/screentest"
)

// undoModel starts Root at 80x24 over fx and returns the test model.
func undoModel(t *testing.T, fx screentest.Fixture, theme string) *teatest.TestModel {
	t.Helper()
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}
	root := tui.New(uc, tui.Config{Locale: domain.LocaleENUS, Theme: theme})
	return teatest.NewTestModel(t, root, teatest.WithInitialTermSize(80, 24))
}

func waitFor(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(want))
	}, teatest.WithDuration(3*time.Second))
}

func finalView(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}

func TestUndoNothingToUndo(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{
		Name: "Checking", Kind: "checking", Currency: "EUR",
		OpeningBalance: "1250.00", OpenedOn: "2026-01-01",
	})
	tm := undoModel(t, fx, "mocha")
	waitFor(t, tm, "net of loans")
	tm.Type("u")
	waitFor(t, tm, "nothing to undo")
	if v := ansi.Strip(finalView(t, tm)); strings.Contains(v, "y/enter yes") {
		t.Errorf("modal opened with nothing to undo:\n%s", v)
	}
}

func TestUndoConfirmApplies(t *testing.T) {
	fx := newSnapshotFixture(t)
	tm := undoModel(t, fx, "mocha")
	tm.Type("2")
	waitFor(t, tm, "booked")
	tm.Type("u")
	waitFor(t, tm, "Undo tx.add")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitFor(t, tm, "Undone tx.add")
	_ = finalView(t, tm)

	if _, err := fx.Service.PlanUndo(t.Context()); err == nil {
		t.Error("journal still has an undoable entry after confirming")
	}
}

func TestUndoEscCancels(t *testing.T) {
	fx := newSnapshotFixture(t)
	tm := undoModel(t, fx, "mocha")
	tm.Type("u")
	waitFor(t, tm, "Undo tx.add")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	if v := ansi.Strip(finalView(t, tm)); strings.Contains(v, "y/enter yes") || strings.Contains(v, "Undone") {
		t.Errorf("modal still open or applied after esc:\n%s", v)
	}
	if _, err := fx.Service.PlanUndo(t.Context()); err != nil {
		t.Errorf("entry was undone despite cancel: %v", err)
	}
}

func TestUndoKeyTypesInSearch(t *testing.T) {
	tm := undoModel(t, newSnapshotFixture(t), "mocha")
	tm.Type("2")
	waitFor(t, tm, "booked")
	tm.Type("/u")
	v := ansi.Strip(finalView(t, tm))
	if strings.Contains(v, "y/enter yes") {
		t.Errorf("undo modal opened while typing in the search box:\n%s", v)
	}
}

func TestUndoListedInHelp(t *testing.T) {
	tm := undoModel(t, screentest.New(t), "mocha")
	tm.Type("?")
	waitFor(t, tm, "undo")
	if v := ansi.Strip(finalView(t, tm)); !strings.Contains(v, "undo") {
		t.Errorf("help does not list undo:\n%s", v)
	}
}

// undoModalSnapshot opens the undo confirmation and diffs the 80x24 frame converted to profile p.
func undoModalSnapshot(t *testing.T, theme string, p colorprofile.Profile) {
	t.Helper()
	tm := undoModel(t, newSnapshotFixture(t), theme)
	tm.Type("u")
	waitFor(t, tm, "y/enter yes")
	golden.RequireEqual(t, []byte(screentest.Downsample(finalView(t, tm), p)))
}

func TestUndoModalSnapshotLatte(t *testing.T) {
	undoModalSnapshot(t, "latte", colorprofile.TrueColor)
}

func TestUndoModalSnapshotMocha(t *testing.T) {
	undoModalSnapshot(t, "mocha", colorprofile.TrueColor)
}

func TestUndoModalSnapshotTerminal(t *testing.T) {
	undoModalSnapshot(t, "terminal", colorprofile.TrueColor)
}

func TestUndoModalSnapshotHighContrast(t *testing.T) {
	undoModalSnapshot(t, "high-contrast", colorprofile.TrueColor)
}

func TestUndoModalSnapshotNoColor(t *testing.T) {
	undoModalSnapshot(t, "mocha", colorprofile.Ascii)
}
