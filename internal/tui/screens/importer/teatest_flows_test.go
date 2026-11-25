package importer_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/importer"
	"github.com/0xballo/fui/internal/tui/theme"
)

func waitFor(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(3*time.Second))
}

func enter(tm *teatest.TestModel) { tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) }

// startFlow runs m under teatest at 80x24, waits for the connector suggestions and opens the
// source form.
func startFlow(t *testing.T, m *importer.Model) *teatest.TestModel {
	t.Helper()
	model := &screenModel{scr: screens.Screen(m), width: 80, height: 24}
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
	waitFor(t, tm, "No import in progress.")
	typeRunes(tm, "n")
	waitFor(t, tm, "> csv")
	return tm
}

// planRich types the rich stream's connector and file and submits the source form.
func planRich(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	typeRunes(tm, "fui")
	enter(tm) // accept the suggestion
	enter(tm) // file
	typeRunes(tm, "rich.fui.jsonl")
	for range 4 {
		enter(tm)
	}
	waitFor(t, tm, "Import preview")
}

func finish(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View()
}

func richFlowModel(t *testing.T, mods *recordingModules) *importer.Model {
	t.Helper()
	fx := newFixture()
	addAccount(t, fx, "Account A")
	addAccountKind(t, fx, "Account B", "investment") // security operations need an investment account
	fx.connectors.fixtures["fui"] = richStream()
	m := importer.New(fx, domain.LocaleENUS, theme.Default())
	m.SetModules(module.Default(), mods)
	return m
}

func TestTeatestAcceptTransfer(t *testing.T) {
	tm := startFlow(t, richFlowModel(t, &recordingModules{}))
	planRich(t, tm)
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	waitFor(t, tm, "Transfer proposals")
	for range 5 { // Errors, error, Warnings, warning, Transfer proposals, proposal
		typeRunes(tm, "j")
	}
	typeRunes(tm, "t")
	waitFor(t, tm, "[x] line")
	enter(tm)
	waitFor(t, tm, "Confirm import")
	typeRunes(tm, "y")
	waitFor(t, tm, "Import complete")
	if view := finish(t, tm); !strings.Contains(view, "transfers=1") {
		t.Fatalf("expected one transfer written:\n%s", view)
	}
}

func TestTeatestEnableModule(t *testing.T) {
	mods := &recordingModules{}
	tm := startFlow(t, richFlowModel(t, mods))
	planRich(t, tm)
	typeRunes(tm, "m")
	waitFor(t, tm, "Disabled modules")
	typeRunes(tm, " ")
	waitFor(t, tm, "[x] enable")
	enter(tm)
	waitFor(t, tm, "(enabling 1)")
	enter(tm)
	waitFor(t, tm, "Confirm import")
	typeRunes(tm, "y")
	waitFor(t, tm, "Import complete")
	finish(t, tm)
	if len(mods.calls) != 1 || !containsModule(mods.calls[0], module.Investments) {
		t.Fatalf("module writer calls = %v", mods.calls)
	}
}

func TestTeatestMapCSVColumnsAndSave(t *testing.T) {
	profiles := &recordingProfiles{}
	m := importer.New(newFixture(), domain.LocaleENUS, theme.Default())
	m.SetProfiles(profiles)
	tm := startFlow(t, m)
	enter(tm) // connector -> file
	typeRunes(tm, "bank.csv")
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	waitFor(t, tm, "Map CSV columns")
	typeRunes(tm, "mybank")
	for range 5 {
		enter(tm)
	}
	waitFor(t, tm, "saved and selected")
	finish(t, tm)
	if len(profiles.saved) != 1 || profiles.saved[0].Name != "mybank" || profiles.saved[0].AmountColumn != "Amount" {
		t.Fatalf("saved = %+v", profiles.saved)
	}
}
