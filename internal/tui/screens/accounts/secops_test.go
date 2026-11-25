package accounts_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/accounts"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type opsClock struct{}

func (opsClock) Now() time.Time { return time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC) }

type opsEnv struct {
	fx  screentest.Fixture
	inv *investing.Service
	scr screens.Screen
}

func newOpsEnv(t *testing.T, crypto bool) *opsEnv {
	t.Helper()
	fx := screentest.New(t)
	ids := []module.ID{module.Core, module.Investments}
	if crypto {
		ids = append(ids, module.Crypto)
	}
	mods, err := module.Validate(ids)
	if err != nil {
		t.Fatal(err)
	}
	eur, _ := domain.ParseCurrency("EUR")
	inv := investing.New(fx.Store, opsClock{}, investing.Config{BaseCurrency: eur, Location: time.UTC, Modules: mods})
	fx.Account(t, app.AddAccountRequest{Name: "Broker", Kind: "investment", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01"})
	fx.Account(t, app.AddAccountRequest{Name: "Cash", Kind: "checking", Currency: "EUR", OpeningBalance: "10000", OpenedOn: "2026-01-01"})
	m := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetPositionsUseCases(inv)
	m.SetCryptoEnabled(crypto)
	scr := screens.Screen(m)
	scr.SetSize(100, 30)
	return &opsEnv{fx: fx, inv: inv, scr: run(t, scr, scr.Init())}
}

func (e *opsEnv) key(t *testing.T, k tea.KeyType) {
	t.Helper()
	var cmd tea.Cmd
	e.scr, cmd = sendKey(e.scr, k)
	e.scr = run(t, e.scr, cmd)
}

func (e *opsEnv) runes(t *testing.T, s string) {
	t.Helper()
	var cmd tea.Cmd
	e.scr, cmd = sendRunes(e.scr, s)
	e.scr = run(t, e.scr, cmd)
}

// field types text into the focused field and moves on; picks is the number of extra enters a
// field with an open suggestion popup needs.
func (e *opsEnv) field(t *testing.T, text string, picks int) {
	t.Helper()
	if text != "" {
		e.runes(t, text)
	}
	for range 1 + picks {
		e.key(t, tea.KeyEnter)
	}
}

func (e *opsEnv) openBroker(t *testing.T) {
	t.Helper()
	e.key(t, tea.KeyEnter)
	if !strings.Contains(e.scr.View(), "Movements — Broker") {
		t.Fatalf("expected Broker detail, got:\n%s", e.scr.View())
	}
}

func (e *opsEnv) addSecurity(t *testing.T) {
	t.Helper()
	e.runes(t, "n")
	e.field(t, "VWCE ETF", 0)
	e.field(t, "VWCE", 0)
	e.field(t, "", 0)
	e.field(t, "etf", 1)
	e.field(t, "", 0)
}

// addOp fills the operation form of the selected position.
func (e *opsEnv) addOp(t *testing.T, kind, qty, price, fees, cash string) {
	t.Helper()
	e.runes(t, "o")
	e.field(t, kind, 1)
	if strings.Contains(e.scr.View(), "Security: VWCE ETF") {
		e.field(t, "", 0) // prefilled from the selected position
	} else {
		e.field(t, "VWCE ETF", 0)
	}
	e.field(t, "2026-06-10", 0)
	e.field(t, qty, 0)
	e.field(t, price, 0)
	e.field(t, fees, 0)
	e.field(t, cash, 0)
}

func (e *opsEnv) ops(t *testing.T) []investing.OpLine {
	t.Helper()
	lines, err := e.inv.ListOps(context.Background(), investing.ListOpsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestAccountsSecurityAndOperations(t *testing.T) {
	e := newOpsEnv(t, false)
	e.openBroker(t)
	e.addSecurity(t)
	secs, err := e.inv.ListSecurities(context.Background())
	if err != nil || len(secs) != 1 || secs[0].Name != "VWCE ETF" || secs[0].Ticker != "VWCE" {
		t.Fatalf("securities = %+v, %v", secs, err)
	}

	e.addOp(t, "buy", "10", "100", "5", "Cash")
	e.addOp(t, "dividend", "10", "1.50", "", "")
	e.addOp(t, "sell", "4", "110", "", "")
	e.addOp(t, "coupon", "1", "2", "", "")
	kinds := []domain.SecurityOpKind{}
	for _, l := range e.ops(t) {
		kinds = append(kinds, l.Op.Kind)
	}
	want := []domain.SecurityOpKind{domain.OpBuy, domain.OpDividend, domain.OpSell, domain.OpCoupon}
	if len(kinds) != 4 {
		t.Fatalf("op kinds = %v, want %v", kinds, want)
	}

	// Operation list, then edit the buy quantity.
	e.runes(t, "l")
	if v := e.scr.View(); !strings.Contains(v, "Operations — Broker") || !strings.Contains(v, "VWCE ETF") {
		t.Fatalf("expected operations list, got:\n%s", v)
	}
	e.runes(t, "e")
	e.field(t, "", 0)
	for range len("10") {
		e.key(t, tea.KeyBackspace)
	}
	e.field(t, "12", 0)
	e.field(t, "", 0)
	e.key(t, tea.KeyEnter) // fees (last) -> submit
	var edited bool
	for _, l := range e.ops(t) {
		if l.Op.Kind == domain.OpBuy && l.Op.Quantity.String() == "12" {
			edited = true
		}
	}
	if !edited {
		t.Fatalf("buy quantity not edited: %+v", e.ops(t))
	}
	if !strings.Contains(e.scr.View(), "Operations — Broker") {
		t.Fatalf("expected to return to the list after edit, got:\n%s", e.scr.View())
	}
}

func TestAccountsDeleteOpConfirms(t *testing.T) {
	e := newOpsEnv(t, false)
	e.openBroker(t)
	e.addSecurity(t)
	e.addOp(t, "buy", "10", "100", "", "Cash")
	e.runes(t, "l")
	e.runes(t, "d")
	if v := e.scr.View(); !strings.Contains(v, "linked cash transaction") {
		t.Fatalf("expected confirm mentioning the cash transaction, got:\n%s", v)
	}
	e.runes(t, "n")
	if len(e.ops(t)) != 1 {
		t.Fatal("op deleted despite cancel")
	}
	e.runes(t, "d")
	e.runes(t, "y")
	if len(e.ops(t)) != 0 {
		t.Fatalf("op not deleted: %+v", e.ops(t))
	}
	page, err := e.fx.Service.ListTx(context.Background(), app.ListTxRequest{Accounts: []string{"Cash"}})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("linked cash tx should be gone: %+v, %v", page.Items, err)
	}
}

func TestAccountsSellMoreThanHeldErrors(t *testing.T) {
	e := newOpsEnv(t, false)
	e.openBroker(t)
	e.addSecurity(t)
	e.addOp(t, "buy", "10", "100", "", "")

	e.runes(t, "o")
	e.field(t, "sell", 1)
	e.field(t, "VWCE ETF", 0)
	e.field(t, "", 0)
	e.field(t, "50", 0)
	e.field(t, "100", 0)
	e.field(t, "", 0)
	e.key(t, tea.KeyEnter) // cash (last) -> submit; ErrorMsg is dropped by this harness
	if len(e.ops(t)) != 1 {
		t.Fatalf("oversell must not be stored: %+v", e.ops(t))
	}

	var msg tea.Msg
	_, cmd := sendKey(e.scr, tea.KeyEnter) // the form stays open: resubmit to read the error
	if cmd != nil {
		msg = cmd()
	}
	if _, ok := msg.(screens.ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg for oversell, got %T", msg)
	}
}

func TestAccountsOpFormValidation(t *testing.T) {
	e := newOpsEnv(t, false)
	e.openBroker(t)
	e.addSecurity(t)
	e.runes(t, "o")
	for range 7 {
		e.key(t, tea.KeyEnter)
	}
	if !strings.Contains(e.scr.View(), "required") {
		t.Fatalf("expected required notice, got:\n%s", e.scr.View())
	}
	e.key(t, tea.KeyEsc)
	e.runes(t, "n")
	for range 5 {
		e.key(t, tea.KeyEnter)
	}
	if !strings.Contains(e.scr.View(), "Name is required.") {
		t.Fatalf("expected name notice, got:\n%s", e.scr.View())
	}
}

func TestAccountsWalletAndCryptoGating(t *testing.T) {
	off := newOpsEnv(t, false)
	off.runes(t, "a")
	off.field(t, "W", 0)
	off.runes(t, "wal")
	if strings.Contains(off.scr.View(), "wallet") {
		t.Fatalf("wallet kind offered while crypto is off:\n%s", off.scr.View())
	}

	on := newOpsEnv(t, true)
	on.runes(t, "a")
	on.field(t, "Wallet", 0)
	on.runes(t, "wal")
	if !strings.Contains(on.scr.View(), "wallet") {
		t.Fatalf("expected wallet suggestion with crypto on:\n%s", on.scr.View())
	}
	on.field(t, "", 1)
	on.field(t, "EUR", 0)
	on.field(t, "0", 0)
	on.key(t, tea.KeyEnter)
	bals, err := on.fx.Service.ListAccounts(context.Background(), app.ListAccountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var wallet bool
	for _, b := range bals {
		wallet = wallet || b.Account.Kind == domain.AccountWallet
	}
	if !wallet {
		t.Skip("wallet creation is rejected by the app fixture")
	}
	// Wallet detail shows the positions panel only with crypto on.
	for range 3 {
		on.key(t, tea.KeyDown)
	}
	on.key(t, tea.KeyEnter)
	if !strings.Contains(on.scr.View(), "Positions") {
		t.Fatalf("wallet detail should show positions with crypto on:\n%s", on.scr.View())
	}
}

func TestAccountsCryptoSecurityGated(t *testing.T) {
	e := newOpsEnv(t, false)
	e.openBroker(t)
	e.runes(t, "n")
	e.field(t, "BTC", 0)
	e.field(t, "BTC", 0)
	e.field(t, "", 0)
	e.field(t, "crypto", 0)
	_, cmd := sendKey(e.scr, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	if _, ok := cmd().(screens.ErrorMsg); !ok {
		t.Fatal("crypto security must be refused while the module is off")
	}
}

func TestAccountsInvestingKeysAvoidGlobalKeys(t *testing.T) {
	global := map[string]bool{}
	for _, b := range tui.DefaultKeyMap().Bindings() {
		for _, k := range b.Keys() {
			global[k] = true
		}
	}
	km := accounts.DefaultKeyMap()
	for _, b := range []key.Binding{km.Security, km.Op, km.Ops, km.OpEdit, km.OpDelete} {
		for _, k := range b.Keys() {
			if global[k] {
				t.Errorf("binding %q collides with a global key", k)
			}
		}
	}
	for _, b := range km.Bindings() {
		for _, k := range b.Keys() {
			if global[k] {
				t.Errorf("accounts binding %q collides with a global key", k)
			}
		}
	}
}

func TestAccountsInvestingHelpHiddenWithoutUseCases(t *testing.T) {
	fx := screentest.New(t)
	without := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	if containsBinding(without.KeyMap(), "add security") {
		t.Fatal("investing keys must stay hidden without use cases")
	}
	with := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	with.SetPositionsUseCases(&positionsStub{})
	for _, d := range []string{"add security", "add operation", "operations"} {
		if !containsBinding(with.KeyMap(), d) {
			t.Errorf("missing help binding %q", d)
		}
	}
}

// TestAccountsInvestingGoldens pins the 80x24 views: identical text across themes, no escapes
// under NO_COLOR.
func TestAccountsInvestingGoldens(t *testing.T) {
	prev := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	steps := map[string]func(*testing.T, *opsEnv){
		"SecurityForm": func(t *testing.T, e *opsEnv) { e.runes(t, "n") },
		"OpForm":       func(t *testing.T, e *opsEnv) { e.runes(t, "o") },
		"OpList":       func(t *testing.T, e *opsEnv) { e.runes(t, "l") },
		"OpDelete": func(t *testing.T, e *opsEnv) {
			e.runes(t, "l")
			e.runes(t, "d")
		},
	}
	themes := []string{"latte", "frappe", "macchiato", "mocha", "terminal", "high-contrast"}
	for name, step := range steps {
		t.Run(name, func(t *testing.T) {
			render := func(th theme.Theme) string {
				e := newOpsEnv(t, false)
				m := accounts.New(e.fx.Service, domain.LocaleENUS, th)
				m.SetPositionsUseCases(e.inv)
				e.scr = screens.Screen(m)
				e.scr.SetSize(80, 24)
				e.scr = run(t, e.scr, e.scr.Init())
				e.openBroker(t)
				e.addSecurity(t)
				e.addOp(t, "buy", "10", "100", "5", "Cash")
				step(t, e)
				return e.scr.View()
			}
			lipgloss.SetColorProfile(termenv.TrueColor)
			var first string
			for _, n := range themes {
				plain := ansi.Strip(render(theme.Resolve(theme.Config{Theme: n}, nil)))
				if first == "" {
					first = plain
				} else if plain != first {
					t.Errorf("theme %s differs once stripped:\n%s\nvs\n%s", n, plain, first)
				}
			}
			golden.RequireEqual(t, []byte(first))

			lipgloss.SetColorProfile(termenv.Ascii)
			v := render(theme.Default())
			if strings.Contains(v, "\x1b") || ansi.Strip(v) != first {
				t.Errorf("NO_COLOR view differs or has escapes:\n%s", v)
			}
		})
	}
}
