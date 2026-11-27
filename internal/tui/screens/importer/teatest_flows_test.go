package importer_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/importer"
	"github.com/0xballo/fui/internal/tui/theme"
)

// flow is a running teatest program together with the model it drives.
type flow struct {
	*teatest.TestModel
	m *screenModel
}

// waitFor polls the latest frame for text. The output stream cannot be searched: Bubble Tea v2
// writes only the cells that changed, so text already on screen is not repeated.
func waitFor(t *testing.T, f *flow, text string) {
	t.Helper()
	timeout := time.After(3 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for !strings.Contains(f.m.lastView(), text) {
		select {
		case <-timeout:
			t.Fatalf("%q not shown after 3s; last frame:\n%s", text, f.m.lastView())
		case <-tick.C:
		}
	}
}

func enter(tm interface{ Send(tea.Msg) }) { tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) }

// startFlow runs m under teatest at 80x24, waits for the connector suggestions and opens the
// source form.
func startFlow(t *testing.T, m *importer.Model) *flow {
	t.Helper()
	model := &screenModel{scr: screens.Screen(m), width: 80, height: 24}
	tm := &flow{TestModel: teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24)), m: model}
	waitFor(t, tm, "No import in progress.")
	typeRunes(tm, "n")
	waitFor(t, tm, "> csv")
	return tm
}

// planRich types the rich stream's connector and file and submits the source form.
func planRich(t *testing.T, tm *flow) {
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

func finish(t *testing.T, tm *flow) string {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
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
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
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
	tm.Send(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
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
