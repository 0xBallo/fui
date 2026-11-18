package transactions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

func totalsFixture(t *testing.T) screentest.Fixture {
	t.Helper()
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "1234", Description: "Salary", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-12", Description: "Coffee", Date: "2026-06-02"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Books", Date: "2026-06-03"})
	return fx
}

func TestTotalsCompactAndBig(t *testing.T) {
	fx := totalsFixture(t)
	for _, tc := range []struct {
		name string
		w, h int
		big  bool
		want string
	}{
		{"compact when short", 100, 19, false, "IN +1,234.00 · OUT −42.00 · 3 tx"},
		{"compact when narrow", 30, 30, false, "IN +1,234.00 · OUT −42.00 · 3 tx"},
		{"big when roomy", 100, 30, true, "OUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(fx.Service, domain.LocaleENUS, theme.Default())
			m.SetSize(tc.w, tc.h)
			scr := run(t, screens.Screen(m), m.Init()).(*Model)
			view := ansi.Strip(scr.View())
			if scr.bigTotals() != tc.big {
				t.Fatalf("bigTotals=%v, want %v", scr.bigTotals(), tc.big)
			}
			if !strings.Contains(view, tc.want) {
				t.Fatalf("missing %q in\n%s", tc.want, view)
			}
			if tc.big && strings.Contains(view, "3 tx") {
				t.Fatalf("big layout must not show the compact line:\n%s", view)
			}
		})
	}
}

func TestTotalsFollowFilter(t *testing.T) {
	fx := totalsFixture(t)
	m := New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 19)
	m.req = app.ListTxRequest{Text: "Coffee"}
	scr := run(t, screens.Screen(m), m.Init()).(*Model)
	view := ansi.Strip(scr.View())
	if !strings.Contains(view, "IN +0.00 · OUT −12.00 · 1 tx") {
		t.Fatalf("totals did not follow the filter:\n%s", view)
	}
}

type failingTotals struct{ UseCases }

func (failingTotals) TxTotals(context.Context, app.ListTxRequest) (app.TxTotals, error) {
	return app.TxTotals{}, errors.New("rate missing")
}

func TestTotalsErrorState(t *testing.T) {
	fx := totalsFixture(t)
	m := New(failingTotals{fx.Service}, domain.LocaleENUS, theme.Default())
	m.SetSize(100, 30)
	scr := run(t, screens.Screen(m), m.Init()).(*Model)
	view := ansi.Strip(scr.View())
	if !strings.Contains(view, "totals n/a") || scr.bigTotals() {
		t.Fatalf("expected muted totals n/a:\n%s", view)
	}
	if !strings.Contains(view, "Salary") {
		t.Fatalf("the list must still render:\n%s", view)
	}
}
