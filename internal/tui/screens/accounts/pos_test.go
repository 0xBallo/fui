package accounts_test

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/accounts"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type positionsStub struct {
	linesByAccount map[string][]investing.PositionLine
	planReq        investing.SetPriceRequest
	applyCount     int
}

func (s *positionsStub) ListPositions(_ context.Context, req investing.PositionsRequest) ([]investing.PositionLine, error) {
	return s.linesByAccount[req.Account], nil
}

func (s *positionsStub) PlanSetPrice(_ context.Context, req investing.SetPriceRequest) (investing.SetPricePlan, error) {
	s.planReq = req
	return investing.SetPricePlan{}, nil
}

func (s *positionsStub) ApplySetPrice(context.Context, investing.SetPricePlan) error {
	s.applyCount++
	return nil
}

func TestAccountsInvestmentPositionsAndPriceEntry(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{
		Name: "Broker", Kind: "investment", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})

	positions := &positionsStub{
		linesByAccount: map[string][]investing.PositionLine{
			"Broker": {{
				Account:  domain.Account{Name: "Broker", Kind: domain.AccountInvestment},
				Security: domain.Security{ID: 1, Name: "VWCE ETF"},
				Position: domain.Position{
					Quantity:    mustQuantity(t, "10", 0),
					AvgCost:     mustMoney(t, 100_00, "EUR"),
					HasPrice:    true,
					LastPrice:   mustMoney(t, 110_00, "EUR"),
					MarketValue: mustMoney(t, 1_100_00, "EUR"),
					Unrealized:  mustMoney(t, 100_00, "EUR"),
				},
			}},
		},
	}

	m := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetPositionsUseCases(positions)
	scr := screens.Screen(m)
	scr.SetSize(100, 30)
	scr = run(t, scr, scr.Init())

	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	view := scr.View()
	if !strings.Contains(view, "Positions") || !strings.Contains(view, "VWCE ETF") || !strings.Contains(view, "P&L") {
		t.Fatalf("expected positions table in investment account detail, got:\n%s", view)
	}

	scr, _ = sendRunes(scr, "p")
	if !strings.Contains(scr.View(), "Set manual price") {
		t.Fatalf("expected manual price form, got:\n%s", scr.View())
	}

	scr, _ = sendRunes(scr, "111.50")
	scr, _ = sendKey(scr, tea.KeyEnter) // price -> date
	scr, cmd = sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)

	if positions.applyCount != 1 {
		t.Fatalf("ApplySetPrice calls = %d, want 1", positions.applyCount)
	}
	if positions.planReq.Security != "VWCE ETF" || positions.planReq.Price != "111.50" {
		t.Fatalf("PlanSetPrice req = %+v", positions.planReq)
	}
	if !strings.Contains(scr.View(), "Positions") {
		t.Fatalf("expected to return to detail view after setting price, got:\n%s", scr.View())
	}
}

func TestAccountsPriceEntryRequiresPrice(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{
		Name: "Broker", Kind: "investment", Currency: "EUR", OpeningBalance: "0", OpenedOn: "2026-01-01",
	})
	positions := &positionsStub{
		linesByAccount: map[string][]investing.PositionLine{
			"Broker": {{
				Account:  domain.Account{Name: "Broker", Kind: domain.AccountInvestment},
				Security: domain.Security{ID: 1, Name: "VWCE ETF"},
				Position: domain.Position{Quantity: mustQuantity(t, "1", 0), AvgCost: mustMoney(t, 100_00, "EUR")},
			}},
		},
	}

	m := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetPositionsUseCases(positions)
	scr := screens.Screen(m)
	scr.SetSize(100, 30)
	scr = run(t, scr, scr.Init())

	scr, cmd := sendKey(scr, tea.KeyEnter)
	scr = run(t, scr, cmd)
	scr, _ = sendRunes(scr, "p")
	scr, _ = sendKey(scr, tea.KeyEnter) // price -> date
	scr, _ = sendKey(scr, tea.KeyEnter) // submit with empty price
	if !strings.Contains(scr.View(), "Price is required.") {
		t.Fatalf("expected required-price validation, got:\n%s", scr.View())
	}
	if positions.applyCount != 0 {
		t.Fatalf("ApplySetPrice calls = %d, want 0", positions.applyCount)
	}
}

func TestAccountsPriceBindingHiddenWhenFeatureMissing(t *testing.T) {
	fx := screentest.New(t)
	without := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	if containsBinding(without.KeyMap(), "set price") {
		t.Fatalf("price key should stay hidden when positions use cases are not set")
	}
	with := accounts.New(fx.Service, domain.LocaleENUS, theme.Default())
	with.SetPositionsUseCases(&positionsStub{})
	if !containsBinding(with.KeyMap(), "set price") {
		t.Fatalf("expected price key when positions use cases are set")
	}
}

func containsBinding(bindings []key.Binding, desc string) bool {
	for _, b := range bindings {
		if b.Help().Desc == desc {
			return true
		}
	}
	return false
}

func mustMoney(t *testing.T, minor int64, code string) domain.Money {
	t.Helper()
	cur, err := domain.ParseCurrency(code)
	if err != nil {
		t.Fatalf("ParseCurrency(%q): %v", code, err)
	}
	m, err := domain.NewMoney(minor, cur)
	if err != nil {
		t.Fatalf("NewMoney(%d, %q): %v", minor, code, err)
	}
	return m
}

func mustQuantity(t *testing.T, value string, scale int) domain.Quantity {
	t.Helper()
	q, err := domain.ParseQuantity(value, scale)
	if err != nil {
		t.Fatalf("ParseQuantity(%q, %d): %v", value, scale, err)
	}
	return q
}
