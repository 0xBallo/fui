package importer_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/importer"
	"github.com/0xballo/fui/internal/tui/theme"
	"github.com/0xballo/fui/pkg/fuiproto"
)

func addAccount(t *testing.T, fx fixture, name string) {
	t.Helper()
	plan, err := fx.PlanAddAccount(context.Background(), app.AddAccountRequest{
		Name: name, Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("PlanAddAccount: %v", err)
	}
	if _, err := fx.ApplyAddAccount(context.Background(), plan); err != nil {
		t.Fatalf("ApplyAddAccount: %v", err)
	}
}

func transferTx(ref, id, date string, amount int64, desc string) fuiproto.Record {
	r := fuiTx(ref, id, date, amount, desc)
	r.Transaction.Kind = fuiproto.TransactionKindTransfer
	return r
}

// richStream holds a transfer pair, a new category, a bad row, a balance mismatch and the
// records of the (disabled) investments module.
func richStream() []fuiproto.Record {
	food := fuiTx("Account A", "food", "2026-06-03", -300, "Coffee")
	food.Transaction.Category = "Food"
	return fuiStream("fui", "rich.fui.jsonl", "sha-rich",
		fuiAccount("Account A", "Account A", "EUR"),
		fuiAccount("Account B", "Account B", "EUR"),
		transferTx("Account A", "out", "2026-06-01", -5000, "to B"),
		transferTx("Account B", "in", "2026-06-02", 5000, "from A"),
		food,
		fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &fuiproto.BalanceRecord{Account: "Account A", Date: "2026-06-04", Amount: 123}},
		fuiproto.Record{Type: fuiproto.RecordTypeError, Error: &fuiproto.ErrorRecord{Line: 9, Message: "bad date"}},
		fuiproto.Record{Type: fuiproto.RecordTypeSecurityOp, SecurityOp: &fuiproto.SecurityOpRecord{
			Account: "Account A", Date: "2026-06-05", Kind: fuiproto.SecurityOpKindBuy, Quantity: 1000000, UnitPrice: 100,
			Security: fuiproto.SecurityMeta{Name: "World ETF", Ticker: "VWCE", Kind: fuiproto.SecurityKindETF, Currency: "EUR", QuantityScale: 6},
		}},
	)
}

// richPreview plans richStream through the screen and returns it on the preview.
func richPreview(t *testing.T, th theme.Theme, mods *recordingModules, profiles *recordingProfiles) (fixture, *importer.Model, screens.Screen) {
	t.Helper()
	fx := newFixture()
	addAccount(t, fx, "Account A")
	addAccount(t, fx, "Account B")
	fx.connectors.fixtures["fui"] = richStream()
	m := importer.New(fx, domain.LocaleENUS, th)
	if mods != nil {
		m.SetModules(module.Default(), mods)
	}
	if profiles != nil {
		m.SetProfiles(profiles)
	}
	var scr screens.Screen = m
	scr.SetSize(80, 24)
	scr = run(t, scr, scr.Init())
	scr, cmd := fillSourceForm(scr, "fui", "rich.fui.jsonl", "", "", "")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Import preview") {
		t.Fatalf("expected the preview, got:\n%s", scr.View())
	}
	return fx, m, scr
}

func TestPreviewDetailsSections(t *testing.T) {
	_, _, scr := richPreview(t, theme.Default(), nil, nil)
	scr, _ = sendKey(scr, tea.KeyTab)
	view := scr.View()
	for range 14 {
		scr, _ = sendRunes(scr, "j") // scroll the window to the last section
	}
	view += scr.View()
	for _, want := range []string{
		"Errors (1)", "line 9: source: bad date", "Transfer proposals (1)", "[ ] line", "Balance mismatches (1)",
		"fix:", "Disabled-module records (1)", "investments", "New categories (1)", "Food",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("details missing %q:\n%s", want, view)
		}
	}
}

// moveToTransfer moves the details cursor onto the first transfer proposal.
func moveToTransfer(t *testing.T, scr screens.Screen) screens.Screen {
	t.Helper()
	for range 20 {
		if strings.Contains(strings.Join(cursorLine(scr.View()), "\n"), "<->") {
			return scr
		}
		scr, _ = sendRunes(scr, "j")
	}
	t.Fatalf("no transfer line reachable:\n%s", scr.View())
	return scr
}

func cursorLine(view string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		if strings.HasPrefix(l, "> ") {
			out = append(out, l)
		}
	}
	return out
}

func TestAcceptTransferAndApply(t *testing.T) {
	fx, _, scr := richPreview(t, theme.Default(), &recordingModules{}, nil)
	scr, _ = sendKey(scr, tea.KeyTab)
	scr = moveToTransfer(t, scr)
	scr, _ = sendRunes(scr, "t")
	if !strings.Contains(scr.View(), "[x] line") || !strings.Contains(scr.View(), "transfers=1/1") {
		t.Fatalf("expected the proposal accepted:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "t")
	if !strings.Contains(scr.View(), "transfers=0/1") {
		t.Fatalf("expected the proposal rejected again:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, "t")

	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd := sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "transfers=1") {
		t.Fatalf("expected one transfer written:\n%s", scr.View())
	}
	_ = fx
}

func TestEnableModuleFromDialog(t *testing.T) {
	mods := &recordingModules{}
	_, _, scr := richPreview(t, theme.Default(), mods, nil)
	scr, _ = sendRunes(scr, "m")
	if !strings.Contains(scr.View(), "Disabled modules") || !strings.Contains(scr.View(), "[ ] skip") {
		t.Fatalf("expected the module dialog:\n%s", scr.View())
	}
	scr, _ = sendRunes(scr, " ")
	if !strings.Contains(scr.View(), "[x] enable") {
		t.Fatalf("expected investments to be enabled:\n%s", scr.View())
	}
	scr, _ = sendKey(scr, tea.KeyEnter)
	if !strings.Contains(scr.View(), "(enabling 1)") {
		t.Fatalf("expected the pending module in the summary:\n%s", scr.View())
	}
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, cmd := sendRunes(scr, "y")
	scr = run(t, scr, cmd)
	if !strings.Contains(scr.View(), "Import complete") {
		t.Fatalf("expected the import to complete:\n%s", scr.View())
	}
	if len(mods.calls) != 1 || !containsModule(mods.calls[0], module.Investments) {
		t.Fatalf("module writer calls = %v, want investments enabled", mods.calls)
	}
}

func containsModule(ids []module.ID, id module.ID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestModuleDialogCancelAndNoRecords(t *testing.T) {
	mods := &recordingModules{}
	_, _, scr := richPreview(t, theme.Default(), mods, nil)
	scr, _ = sendRunes(scr, "m")
	scr, _ = sendRunes(scr, " ")
	scr, _ = sendKey(scr, tea.KeyEsc)
	if strings.Contains(scr.View(), "(enabling") {
		t.Fatalf("esc must discard the choice:\n%s", scr.View())
	}

	// Without a writer the dialog cannot enable anything.
	_, _, scr = richPreview(t, theme.Default(), nil, nil)
	scr, _ = sendRunes(scr, "m")
	scr, _ = sendRunes(scr, " ")
	if !strings.Contains(scr.View(), "not available") || strings.Contains(scr.View(), "[x] enable") {
		t.Fatalf("expected a notice, got:\n%s", scr.View())
	}

	fx := newFixture()
	fx.connectors.fixtures["fui"] = fuiStream("fui", "a.fui.jsonl", "sha-plain",
		fuiAccount("Z", "Z", "EUR"), fuiTx("Z", "t", "2026-06-01", -100, "x"))
	var plain screens.Screen = importer.New(fx, domain.LocaleENUS, theme.Default())
	plain.SetSize(80, 24)
	plain = run(t, plain, plain.Init())
	plain, cmd := fillSourceForm(plain, "fui", "a.fui.jsonl", "", "", "")
	plain = run(t, plain, cmd)
	plain, _ = sendRunes(plain, "create")
	plain, _ = sendKey(plain, tea.KeyEnter)
	plain, cmd = sendKey(plain, tea.KeyEnter)
	plain = run(t, plain, cmd)
	plain, _ = sendRunes(plain, "m")
	if !strings.Contains(plain.View(), "No records of disabled modules.") {
		t.Fatalf("expected a notice:\n%s", plain.View())
	}
}

func TestMapCSVColumnsAndSave(t *testing.T) {
	fx := newFixture()
	profiles := &recordingProfiles{}
	m := importer.New(fx, domain.LocaleENUS, theme.Default())
	m.SetProfiles(profiles)
	var scr screens.Screen = m
	scr.SetSize(80, 24)
	scr = run(t, scr, scr.Init())
	scr, _ = sendRunes(scr, "n")

	// A file is needed first.
	scr, cmd := sendKey(scr, tea.KeyCtrlP)
	if cmd != nil || !strings.Contains(scr.View(), "File is required") {
		t.Fatalf("expected the file notice:\n%s", scr.View())
	}

	scr, _ = sendKey(scr, tea.KeyEnter) // connector -> file
	scr, _ = sendRunes(scr, "bank.csv")
	scr, cmd = sendKey(scr, tea.KeyCtrlP)
	scr = run(t, scr, cmd)
	view := scr.View()
	for _, want := range []string{"Map CSV columns", "Salary", "Profile name:", "Memo:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("mapper missing %q:\n%s", want, view)
		}
	}

	// Name the profile, keep every suggested role (Ref stays "other"), save on the last field.
	scr, _ = sendRunes(scr, "mybank")
	for range 5 {
		scr, _ = sendKey(scr, tea.KeyEnter)
	}
	if len(profiles.saved) != 1 {
		t.Fatalf("saved = %+v\n%s", profiles.saved, scr.View())
	}
	p := profiles.saved[0]
	if p.Name != "mybank" || p.DateColumn != "Date" || p.AmountColumn != "Amount" || p.DescriptionColumn != "Memo" || p.ExternalIDColumn != "" || p.Delimiter != "," {
		t.Fatalf("profile = %+v", p)
	}
	if !strings.Contains(scr.View(), "mybank") || !strings.Contains(scr.View(), "saved and selected") {
		t.Fatalf("expected the source form to select the profile:\n%s", scr.View())
	}
}

func TestMapCSVValidation(t *testing.T) {
	fx := newFixture()
	profiles := &recordingProfiles{}
	m := importer.New(fx, domain.LocaleENUS, theme.Default())
	m.SetProfiles(profiles)
	var scr screens.Screen = m
	scr.SetSize(80, 24)
	scr = run(t, scr, scr.Init())
	scr, _ = sendRunes(scr, "n")
	scr, _ = sendKey(scr, tea.KeyEnter)
	scr, _ = sendRunes(scr, "bank.csv")
	scr, cmd := sendKey(scr, tea.KeyCtrlP)
	scr = run(t, scr, cmd)

	scr, _ = sendKey(scr, tea.KeyShiftTab) // wrap to the last field
	scr, _ = sendKey(scr, tea.KeyEnter)    // submit with no name
	if !strings.Contains(scr.View(), "Profile name is required.") {
		t.Fatalf("expected the name notice:\n%s", scr.View())
	}
	for range 4 {
		scr, _ = sendKey(scr, tea.KeyShiftTab)
	}
	scr, _ = sendRunes(scr, "mybank")
	for range 4 {
		scr, _ = sendKey(scr, tea.KeyEnter)
	}
	// Ref -> "date" duplicates the date column.
	for range 5 {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr, _ = sendRunes(scr, "date")
	scr, _ = sendKey(scr, tea.KeyEnter)
	if len(profiles.saved) != 0 || !strings.Contains(scr.View(), "assigned to both") {
		t.Fatalf("expected a duplicate-role notice, saved=%v:\n%s", profiles.saved, scr.View())
	}
	scr, _ = sendKey(scr, tea.KeyEsc)
	if !strings.Contains(scr.View(), "Import") {
		t.Fatalf("esc should return to the source form:\n%s", scr.View())
	}

	// Without a profile port the mapper is not offered.
	var bare screens.Screen = importer.New(fx, domain.LocaleENUS, theme.Default())
	bare = run(t, bare, bare.Init())
	bare, _ = sendRunes(bare, "n")
	bare, _ = sendKey(bare, tea.KeyCtrlP)
	if !strings.Contains(bare.View(), "not available") {
		t.Fatalf("expected a notice:\n%s", bare.View())
	}
}

func TestKeyMapListsNewKeys(t *testing.T) {
	var scr screens.Screen = importer.New(newFixture(), domain.LocaleENUS, theme.Default())
	var help []string
	for _, b := range scr.KeyMap() {
		help = append(help, b.Help().Key)
	}
	got := strings.Join(help, " ")
	for _, want := range []string{"tab", "t", "m", "ctrl+p"} {
		if !strings.Contains(" "+got+" ", " "+want+" ") {
			t.Errorf("help bar lacks %q: %s", want, got)
		}
	}
}

func newImporter(fx fixture, th theme.Theme) *importer.Model {
	return importer.New(fx, domain.LocaleENUS, th)
}
