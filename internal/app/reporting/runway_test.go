package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

func TestAccountKindEnabled(t *testing.T) {
	core := mustModules(t, module.Core)
	withInvestments := mustModules(t, module.Core, module.Investments)
	tests := []struct {
		name string
		set  module.Set
		kind domain.AccountKind
		want bool
	}{
		{"investment off", core, domain.AccountInvestment, false},
		{"investment on", withInvestments, domain.AccountInvestment, true},
		{"checking off", core, domain.AccountChecking, true},
		{"loan off", core, domain.AccountLoan, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accountKindEnabled(tt.set, tt.kind); got != tt.want {
				t.Fatalf("accountKindEnabled = %v, want %v", got, tt.want)
			}
		})
	}
}

// The runway KPI never counts investment accounts as liquidity; the module filter must keep
// that true whether or not the module is on.
func TestRunwayExcludesAccountsOfDisabledModules(t *testing.T) {
	source := &fakeSource{
		accounts: []domain.Account{
			account(t, 1, "Main", domain.AccountChecking, "EUR", false),
			account(t, 2, "Broker", domain.AccountInvestment, "EUR", false),
		},
		balances: map[domain.AccountID]domain.Money{1: money(t, 100_000, "EUR"), 2: money(t, 500_000, "EUR")},
		transactions: []domain.Transaction{
			makeTx(t, 1, date(t, 2026, 8, 10), -100_000, "EUR", domain.KindPayment, 0),
			makeTx(t, 1, date(t, 2026, 9, 10), -100_000, "EUR", domain.KindPayment, 0),
			makeTx(t, 1, date(t, 2026, 10, 10), -100_000, "EUR", domain.KindPayment, 0),
		},
	}
	core := mustModules(t, module.Core)
	withInvestments := mustModules(t, module.Core, module.Investments)
	tests := []struct {
		name string
		mods *module.Set
		want int64
	}{
		{"not wired", nil, 100_000},
		{"investments off", &core, 100_000},
		{"investments on", &withInvestments, 100_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useCase, err := NewRunway(source, fixedClock{now: time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatal(err)
			}
			if tt.mods != nil {
				useCase = useCase.WithModules(*tt.mods)
			}
			got, err := useCase.Run(context.Background(), RunwayParams{MonthStartDay: 1, BaseCurrency: cur(t, "EUR")})
			if err != nil {
				t.Fatal(err)
			}
			if got.Liquidity.Minor() != tt.want {
				t.Fatalf("liquidity = %d, want %d", got.Liquidity.Minor(), tt.want)
			}
		})
	}
}

func mustModules(t *testing.T, ids ...module.ID) module.Set {
	t.Helper()
	set, err := module.Validate(ids)
	if err != nil {
		t.Fatal(err)
	}
	return set
}
