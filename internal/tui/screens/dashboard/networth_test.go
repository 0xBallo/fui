package dashboard_test

import (
	"context"
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/dashboard"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

type netWorthStub struct {
	point reporting.NetWorthPoint
}

func (s netWorthStub) NetWorthAt(context.Context, domain.Date) (reporting.NetWorthPoint, error) {
	return s.point, nil
}

func TestDashboardNetWorthShowsPositionsAndAssetsComponents(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	uc := useCases{Service: fx.Service, Reporting: fx.NewReporting(t)}

	m := dashboard.New(uc, domain.LocaleENUS, theme.Default(), 1)
	m.SetNetWorthUseCases(netWorthStub{point: reporting.NetWorthPoint{
		Positions: &reporting.Amount{Minor: 300_00, Currency: "EUR"},
		Assets:    &reporting.Amount{Minor: 1_500_00, Currency: "EUR"},
	}})
	scr := screens.Screen(m)
	scr.SetSize(100, 30)
	scr = run(t, scr, scr.Init())

	view := scr.View()
	if !strings.Contains(view, "Positions: 300.00") || !strings.Contains(view, "Assets: 1,500.00") {
		t.Fatalf("expected net-worth component lines, got:\n%s", view)
	}
}

func TestGrossNet(t *testing.T) {
	eur, _ := domain.ParseCurrency("EUR")
	line := func(kind domain.AccountKind, minor int64) app.NetWorthLine {
		m, _ := domain.NewMoney(minor, eur)
		return app.NetWorthLine{Account: domain.Account{Kind: kind}, Balance: m, Converted: m}
	}
	tests := []struct {
		name       string
		lines      []app.NetWorthLine
		gross, net int64
	}{
		{"empty", nil, 0, 0},
		{"no loans", []app.NetWorthLine{line(domain.AccountChecking, 1000), line(domain.AccountSavings, 500)}, 1500, 1500},
		{"loans", []app.NetWorthLine{line(domain.AccountChecking, 1000), line(domain.AccountLoan, -400)}, 1000, 600},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gross, net, err := dashboard.GrossNet(app.NetWorth{Base: eur, Lines: tc.lines})
			if err != nil {
				t.Fatal(err)
			}
			if gross.Minor() != tc.gross || net.Minor() != tc.net {
				t.Fatalf("gross/net = %d/%d, want %d/%d", gross.Minor(), net.Minor(), tc.gross, tc.net)
			}
		})
	}
}
