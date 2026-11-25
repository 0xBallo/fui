package lists_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
	"github.com/0xballo/fui/internal/tui/theme"
)

// assetsAdapter satisfies lists.AssetsUseCases like the cmd/fui adapter does.
type assetsAdapter struct {
	*assetting.Service
	loans *lending.Service
}

func (a assetsAdapter) ListLoanNames(ctx context.Context, enabled module.Set) ([]string, error) {
	infos, err := a.loans.List(ctx, enabled)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(infos))
	for i, in := range infos {
		names[i] = in.Loan.Name
	}
	return names, nil
}

type assetsEnv struct {
	fx      screentest.Fixture
	uc      assetsAdapter
	enabled module.Set
}

func newAssetsEnv(t *testing.T) assetsEnv {
	t.Helper()
	fx := screentest.New(t)
	enabled, err := module.Validate([]module.ID{module.Core, module.Recurring, module.Loans, module.Assets})
	if err != nil {
		t.Fatalf("module.Validate: %v", err)
	}
	clk := fixedClock{now: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)}
	eur, _ := domain.ParseCurrency("EUR")
	uc := assetsAdapter{
		Service: assetting.New(fx.Store, clk, stubBackup{}, assetting.Config{Locale: domain.LocaleENUS, Location: time.UTC, BaseCurrency: eur}),
		loans:   lending.New(fx.Store, clk, stubBackup{}, lending.Config{Locale: domain.LocaleENUS}),
	}
	return assetsEnv{fx: fx, uc: uc, enabled: enabled}
}

func (e assetsEnv) screen(t *testing.T, th theme.Theme) screens.Screen {
	t.Helper()
	scr := screens.Screen(lists.NewAssets(e.uc, e.enabled, domain.LocaleENUS, th))
	scr.SetSize(80, 24)
	return run(t, scr, scr.Init())
}

func (e assetsEnv) addAsset(t *testing.T, req assetting.AddRequest) {
	t.Helper()
	p, err := e.uc.PlanAdd(t.Context(), e.enabled, req)
	if err != nil {
		t.Fatalf("PlanAdd: %v", err)
	}
	if _, err := e.uc.ApplyAdd(t.Context(), p); err != nil {
		t.Fatalf("ApplyAdd: %v", err)
	}
}

func (e assetsEnv) addValue(t *testing.T, asset, day, v string) {
	t.Helper()
	p, err := e.uc.PlanValue(t.Context(), e.enabled, assetting.ValueRequest{Asset: asset, Date: day, Value: v})
	if err != nil {
		t.Fatalf("PlanValue: %v", err)
	}
	if err := e.uc.ApplyValue(t.Context(), p); err != nil {
		t.Fatalf("ApplyValue: %v", err)
	}
}

func (e assetsEnv) addLoan(t *testing.T) {
	t.Helper()
	e.fx.Account(t, app.AddAccountRequest{Name: "Main", Kind: "checking", Currency: "EUR", OpeningBalance: "5000", OpenedOn: "2025-01-01"})
	p, err := e.uc.loans.PlanAdd(t.Context(), e.enabled, lending.AddRequest{
		Name: "Mortgage", Kind: "mortgage", Principal: "12000", Disbursed: "2026-01-01", TermMonths: 12,
		RateType: "fixed", Rate: "3,5", Debit: "Main",
	})
	if err != nil {
		t.Fatalf("loan PlanAdd: %v", err)
	}
	if _, err := e.uc.loans.ApplyAdd(t.Context(), p); err != nil {
		t.Fatalf("loan ApplyAdd: %v", err)
	}
}

// typeText types s into the focused field.
func typeText(scr screens.Screen, s string) screens.Screen {
	scr, _ = sendRunes(scr, s)
	return scr
}

// enter presses Enter and runs the resulting command chain.
func enter(t *testing.T, scr screens.Screen) screens.Screen {
	t.Helper()
	scr, cmd := sendKey(scr, tea.KeyEnter)
	return run(t, scr, cmd)
}

func press(t *testing.T, scr screens.Screen, k string) screens.Screen {
	t.Helper()
	scr, cmd := sendRunes(scr, k)
	return run(t, scr, cmd)
}

func wantView(t *testing.T, scr screens.Screen, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(scr.View(), w) {
			t.Fatalf("missing %q in:\n%s", w, scr.View())
		}
	}
}

func dontWantView(t *testing.T, scr screens.Screen, w string) {
	t.Helper()
	if strings.Contains(scr.View(), w) {
		t.Fatalf("unexpected %q in:\n%s", w, scr.View())
	}
}

func TestAssetsChrome(t *testing.T) {
	e := newAssetsEnv(t)
	a := lists.NewAssets(e.uc, e.enabled, domain.LocaleENUS, theme.Default())
	if a.Title() != "Assets" || len(a.KeyMap()) != 10 || a.InputActive() {
		t.Fatalf("unexpected chrome: title=%q keys=%d", a.Title(), len(a.KeyMap()))
	}
}

func TestAssetsAdd(t *testing.T) {
	e := newAssetsEnv(t)
	scr := e.screen(t, theme.Default())
	scr = press(t, scr, "a")
	wantView(t, scr, "Add asset")
	scr = enter(t, typeText(scr, "Flat"))
	scr = typeText(scr, "property")
	scr = enter(t, scr) // accept suggestion
	scr = enter(t, scr) // -> date
	scr = enter(t, typeText(scr, "2026-01-10"))
	scr = enter(t, typeText(scr, "250000"))
	scr = enter(t, scr) // currency left empty: submit
	wantView(t, scr, "Flat", "property", "250,000.00 EUR")
}

func TestAssetsAddErrorShownInline(t *testing.T) {
	e := newAssetsEnv(t)
	scr := press(t, e.screen(t, theme.Default()), "a")
	for range 4 {
		scr = enter(t, scr)
	}
	scr = enter(t, scr)
	wantView(t, scr, "Add asset", "Error:")
}

func TestAssetsEdit(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000", PurchaseDate: "2026-01-10"})
	scr := e.screen(t, theme.Default())
	scr = press(t, scr, "e")
	wantView(t, scr, "Edit asset")
	for range "Flat" {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr = enter(t, typeText(scr, "House"))
	scr = enterUntilClosed(t, scr, "Edit asset")
	wantView(t, scr, "House")
	dontWantView(t, scr, "Flat")
}

func TestAssetsEditUnchangedClosesForm(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	scr := press(t, e.screen(t, theme.Default()), "e")
	scr = enterUntilClosed(t, scr, "Edit asset")
	wantView(t, scr, "Flat")
}

func TestAssetsFormEscAndShiftTab(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	scr := press(t, e.screen(t, theme.Default()), "a")
	if !scr.InputActive() {
		t.Fatal("form should capture input")
	}
	scr = enter(t, typeText(scr, "X"))
	scr, _ = sendKey(scr, tea.KeyShiftTab)
	scr, _ = sendKey(scr, tea.KeyEsc)
	if scr.InputActive() {
		t.Fatal("esc should close the form")
	}
	wantView(t, scr, "Flat")
}

func TestAssetsDetailValuationsAndDelete(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000", PurchaseDate: "2026-01-10"})
	scr := e.screen(t, theme.Default())

	scr = press(t, scr, "v")
	wantView(t, scr, "Add valuation: Flat")
	scr = enter(t, typeText(scr, "2026-03-01"))
	scr = enter(t, typeText(scr, "1200"))
	wantView(t, scr, "1,200.00 EUR")

	scr = enter(t, scr) // open detail
	wantView(t, scr, "Asset: Flat", "Valuation history", "2026-03-01")

	// A second valuation on the same date asks before replacing.
	scr = press(t, scr, "v")
	scr = enter(t, typeText(scr, "2026-03-01"))
	scr = enter(t, typeText(scr, "1300"))
	wantView(t, scr, "Replace valuation", "previous value: 1,200.00 EUR")
	scr = press(t, scr, "y")
	wantView(t, scr, "Asset: Flat", "1,300.00 EUR")

	scr = press(t, scr, "d")
	wantView(t, scr, "Delete valuation")
	scr = press(t, scr, "n")
	wantView(t, scr, "2026-03-01")
	scr = press(t, scr, "d")
	scr = press(t, scr, "y")
	dontWantView(t, scr, "2026-03-01")

	scr, _ = sendKey(scr, tea.KeyEsc)
	wantView(t, scr, "Kind", "Flat")
}

func TestAssetsDetailEditAndNav(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000", PurchaseDate: "2026-01-10"})
	e.addValue(t, "Flat", "2026-02-01", "1100")
	e.addValue(t, "Flat", "2026-03-01", "1200")
	scr := enter(t, e.screen(t, theme.Default()))
	scr = press(t, scr, "j")
	scr = press(t, scr, "k")
	scr = press(t, scr, "e")
	for range "Flat" {
		scr, _ = sendKey(scr, tea.KeyBackspace)
	}
	scr = enter(t, typeText(scr, "House"))
	scr = enterUntilClosed(t, scr, "Edit asset")
	wantView(t, scr, "Asset: House")
}

func TestAssetsDeleteWithConfirm(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	e.addAsset(t, assetting.AddRequest{Name: "Car", Kind: "vehicle", Value: "500"})
	scr := e.screen(t, theme.Default())
	scr = press(t, scr, "j")
	scr = press(t, scr, "k")
	scr = press(t, scr, "d")
	wantView(t, scr, "Delete asset", "valuation(s) will be removed")
	scr = press(t, scr, "n")
	wantView(t, scr, "Car", "Flat")
	scr = press(t, scr, "d")
	scr = press(t, scr, "y")
	dontWantView(t, scr, "Flat")
	wantView(t, scr, "Car")
}

func TestAssetsLinkUnlink(t *testing.T) {
	e := newAssetsEnv(t)
	e.addLoan(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "20000", Currency: "EUR"})
	scr := e.screen(t, theme.Default())

	scr = press(t, scr, "l")
	wantView(t, scr, "Link loan: Flat")
	scr = typeText(scr, "Mort")
	scr = enter(t, scr) // accept suggestion
	scr = enter(t, scr)
	wantView(t, scr, "Mortgage")
	dontWantView(t, scr, "Error:")

	scr = press(t, scr, "L")
	wantView(t, scr, "Unlink loan")
	scr = press(t, scr, "n")
	wantView(t, scr, "Mortgage")
	scr = press(t, scr, "L")
	scr = press(t, scr, "y")
	dontWantView(t, scr, "Mortgage")
}

func TestAssetsErrorsShownInline(t *testing.T) {
	e := newAssetsEnv(t)
	e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "1000"})
	scr := e.screen(t, theme.Default())
	scr = press(t, scr, "L") // not linked
	wantView(t, scr, "Error:")
	scr = press(t, scr, "l") // no loans to pick: the form still opens, linking fails
	scr = enter(t, typeText(scr, "Nope"))
	wantView(t, scr, "Error:")
	scr, _ = sendKey(scr, tea.KeyEsc)

	scr = enter(t, scr)
	scr = press(t, scr, "d") // no valuations to delete
	scr, _ = sendKey(scr, tea.KeyEsc)
	scr = press(t, scr, "v")
	scr = enter(t, scr)
	scr = enter(t, scr)
	wantView(t, scr, "Error:")
}

func TestAssetsEmptyListKeys(t *testing.T) {
	e := newAssetsEnv(t)
	scr := e.screen(t, theme.Default())
	for _, k := range []string{"e", "d", "v", "l", "L"} {
		scr = press(t, scr, k)
	}
	scr = enter(t, scr)
	wantView(t, scr, "Assets")
}

func TestAssetsSnapshots(t *testing.T) {
	themes := []struct {
		name    string
		theme   string
		profile termenv.Profile
	}{
		{"latte", "latte", termenv.TrueColor},
		{"mocha", "mocha", termenv.TrueColor},
		{"terminal", "terminal", termenv.TrueColor},
		{"high-contrast", "high-contrast", termenv.TrueColor},
		{"no-color", "mocha", termenv.Ascii},
	}
	for _, tc := range themes {
		t.Run(tc.name, func(t *testing.T) {
			r := lipgloss.DefaultRenderer()
			prev := r.ColorProfile()
			r.SetColorProfile(tc.profile)
			t.Cleanup(func() { r.SetColorProfile(prev) })

			e := newAssetsEnv(t)
			e.addLoan(t)
			e.addAsset(t, assetting.AddRequest{Name: "Flat", Kind: "property", Value: "20000", PurchaseDate: "2026-01-10", Loan: "Mortgage"})
			e.addAsset(t, assetting.AddRequest{Name: "Car", Kind: "vehicle", Value: "9000", PurchaseDate: "2026-02-01"})
			e.addValue(t, "Flat", "2026-04-01", "21000")
			th := theme.Resolve(theme.Config{Theme: tc.theme}, nil)
			list := e.screen(t, th).View()
			detail := enter(t, e.screen(t, th)).View()
			form, _ := sendRunes(e.screen(t, th), "a")
			golden.RequireEqual(t, []byte(list+"\n---\n"+detail+"\n---\n"+form.View()))
		})
	}
}

// enterUntilClosed presses Enter until the form titled title is gone, so tests do not depend on
// how many suggestion dropdowns the prefilled fields open on the way.
func enterUntilClosed(t *testing.T, scr screens.Screen, title string) screens.Screen {
	t.Helper()
	for range 12 {
		if !strings.Contains(scr.View(), title) {
			return scr
		}
		scr = enter(t, scr)
	}
	t.Fatalf("form %q did not close:\n%s", title, scr.View())
	return scr
}

func TestAssetsKeysDoNotCollideWithGlobalKeys(t *testing.T) {
	global := map[string]string{}
	for _, b := range tui.DefaultKeyMap().Bindings() {
		for _, k := range b.Keys() {
			global[k] = b.Help().Desc
		}
	}
	for _, b := range lists.DefaultKeyMap().BindingsWithAssets() {
		for _, k := range b.Keys() {
			if desc, ok := global[k]; ok {
				t.Errorf("asset key %q (%s) collides with global %q", k, b.Help().Desc, desc)
			}
		}
	}
}
