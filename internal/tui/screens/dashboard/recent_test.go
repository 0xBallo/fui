package dashboard

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

func mustPeriod(t *testing.T) domain.Period {
	t.Helper()
	from, _ := domain.NewDate(2026, 3, 1)
	to, _ := domain.NewDate(2026, 3, 31)
	p, err := domain.NewPeriod(from, to)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func txView(t *testing.T, minor int64, st domain.TxStatus, transfer bool) app.TxView {
	t.Helper()
	cur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	amt, _ := domain.NewMoney(minor, cur)
	d, _ := domain.NewDate(2026, 3, 5)
	v := app.TxView{Transaction: domain.Transaction{Date: d, Amount: amt, Status: st, Description: "desc"}}
	if transfer {
		v.Transfer = &domain.TransferGroup{}
	}
	return v
}

func loaded(t *testing.T, items []app.TxView) *recentSection {
	t.Helper()
	s := newRecentSection(nil, domain.LocaleENUS, theme.Default())
	p := mustPeriod(t)
	s.period = p
	if _, ok := s.update(recentMsg{period: p, items: items}); !ok {
		t.Fatal("recentMsg not consumed")
	}
	return s
}

func TestRecentRowStyles(t *testing.T) {
	th := theme.Default()
	s := loaded(t, []app.TxView{
		txView(t, -500, domain.StatusBooked, false),
		txView(t, 500, domain.StatusBooked, false),
		txView(t, -500, domain.StatusPending, false),
		txView(t, -500, domain.StatusBooked, true),
	})
	want := []theme.Flow{theme.FlowExpense, theme.FlowIncome, theme.FlowExpense, theme.FlowTransfer}
	pending := []bool{false, false, true, false}
	rows := s.rows()
	for i, r := range rows {
		got := r.Style.GetForeground()
		if w := th.FlowStyle(want[i], pending[i]).GetForeground(); got != w {
			t.Errorf("row %d foreground = %v, want %v", i, got, w)
		}
	}
	if v := s.view(100); !strings.Contains(v, "View all transactions (t)") {
		t.Errorf("view lacks CTA:\n%s", v)
	}
}

func TestRecentLoadLimit(t *testing.T) {
	var got app.ListTxRequest
	uc := listTxStub{f: func(req app.ListTxRequest) (app.TxPage, error) {
		got = req
		items := make([]app.TxView, 0, recentLimit)
		for i := 0; i < recentLimit; i++ {
			items = append(items, txView(t, int64(-i-1), domain.StatusBooked, false))
		}
		return app.TxPage{Items: items}, nil
	}}
	s := newRecentSection(uc, domain.LocaleENUS, theme.Default())
	p := mustPeriod(t)
	msg := s.load(sectionParams{Period: p})()
	s.update(msg)
	want := app.ListTxRequest{From: "2026-03-01", To: "2026-03-31", Limit: 10}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("request mismatch (-want +got):\n%s", diff)
	}
	if n := len(s.rows()); n != 10 {
		t.Errorf("rows = %d, want 10", n)
	}
}

type listTxStub struct {
	UseCases
	f func(app.ListTxRequest) (app.TxPage, error)
}

func (s listTxStub) ListTx(_ context.Context, req app.ListTxRequest) (app.TxPage, error) {
	return s.f(req)
}

func TestRecentKeyAndEmpty(t *testing.T) {
	s := loaded(t, nil)
	if v := s.view(80); !strings.Contains(v, "no transactions in this period") {
		t.Errorf("empty view = %q", v)
	}
	cmd, ok := s.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !ok || cmd == nil {
		t.Fatal("t not handled")
	}
	if got := cmd(); got != (screens.SwitchScreenMsg{Target: screens.TabTransactions}) {
		t.Errorf("msg = %#v", got)
	}
	if _, ok := s.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}); ok {
		t.Error("x consumed")
	}
}
