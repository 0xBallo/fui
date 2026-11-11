package reporting

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/0xballo/fui/internal/domain"
)

func TestNewUseCasesRequireDependencies(t *testing.T) {
	clock := fixedClock{now: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)}
	source := &fakeSource{}
	if _, err := NewSpending(nil, clock); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("spending missing source err = %v", err)
	}
	if _, err := NewCashFlow(source, nil); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("cashflow missing clock err = %v", err)
	}
	if _, err := NewRunway(nil, nil); !errors.Is(err, domain.ErrMissingField) {
		t.Fatalf("runway missing deps err = %v", err)
	}
}

func TestSpendingRunWithDefaultPeriodAndTimezone(t *testing.T) {
	utcNow := time.Date(2026, 11, 26, 23, 30, 0, 0, time.UTC)
	kiritimati, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Fatal(err)
	}
	data := &fakeSource{
		transactions: []domain.Transaction{
			makeTx(t, 1, date(t, 2026, 11, 26), -10_00, "EUR", domain.KindPayment, 1),
			makeTx(t, 1, date(t, 2026, 11, 27), -20_00, "EUR", domain.KindPayment, 1),
		},
	}
	tests := []struct {
		name     string
		now      time.Time
		wantFrom domain.Date
		wantTo   domain.Date
		wantSum  int64
	}{
		{
			name:     "utc",
			now:      utcNow,
			wantFrom: date(t, 2026, 10, 27),
			wantTo:   date(t, 2026, 11, 26),
			wantSum:  10_00,
		},
		{
			name:     "kiritimati",
			now:      utcNow.In(kiritimati),
			wantFrom: date(t, 2026, 11, 27),
			wantTo:   date(t, 2026, 12, 26),
			wantSum:  20_00,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useCase, err := NewSpending(data, fixedClock{now: tt.now})
			if err != nil {
				t.Fatal(err)
			}
			result, err := useCase.Run(context.Background(), SpendingParams{
				MonthStartDay: 27,
				BaseCurrency:  cur(t, "EUR"),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Period.From() != tt.wantFrom || result.Period.To() != tt.wantTo {
				t.Fatalf("period = [%v,%v], want [%v,%v]", result.Period.From(), result.Period.To(), tt.wantFrom, tt.wantTo)
			}
			if result.Total.Minor() != tt.wantSum {
				t.Fatalf("total = %d, want %d", result.Total.Minor(), tt.wantSum)
			}
		})
	}
}

func TestCashFlowRun(t *testing.T) {
	period := period(t, date(t, 2026, 11, 1), date(t, 2026, 12, 31))
	useCase, err := NewCashFlow(&fakeSource{
		transactions: []domain.Transaction{
			makeTx(t, 1, date(t, 2026, 11, 2), -10_000, "EUR", domain.KindPayment, 0),
			makeTx(t, 1, date(t, 2026, 11, 3), 20_000, "EUR", domain.KindInterest, 0),
			makeTx(t, 1, date(t, 2026, 11, 4), -50_00, "USD", domain.KindPayment, 0),
			makeTx(t, 1, date(t, 2026, 12, 3), -1_000, "EUR", domain.KindTransfer, 0),
		},
		rates: map[string][]fxPoint{
			"USD->EUR": {{date: date(t, 2026, 11, 4), rate: rate(t, 900_000_000)}},
		},
	}, fixedClock{now: time.Date(2026, 12, 15, 8, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := useCase.Run(context.Background(), CashFlowParams{
		Period:        period,
		MonthStartDay: 1,
		BaseCurrency:  cur(t, "EUR"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalInflow.Minor() != 20_000 {
		t.Fatalf("inflow = %d", got.TotalInflow.Minor())
	}
	if got.TotalOutflow.Minor() != 14_500 {
		t.Fatalf("outflow = %d", got.TotalOutflow.Minor())
	}
	if got.TotalNet.Minor() != 5_500 {
		t.Fatalf("net = %d", got.TotalNet.Minor())
	}
}

func TestRunwayRun(t *testing.T) {
	source := &fakeSource{
		accounts: []domain.Account{
			account(t, 1, "Wallet", domain.AccountCash, "EUR", false),
			account(t, 2, "Card", domain.AccountCreditCard, "EUR", false),
		},
		balances: map[domain.AccountID]domain.Money{
			1: money(t, 300_000, "EUR"),
			2: money(t, -60_000, "EUR"),
		},
		transactions: []domain.Transaction{
			makeTx(t, 1, date(t, 2026, 8, 10), -100_000, "EUR", domain.KindPayment, 0),
			makeTx(t, 1, date(t, 2026, 9, 10), -100_000, "EUR", domain.KindPayment, 0),
			makeTx(t, 1, date(t, 2026, 10, 10), -100_000, "EUR", domain.KindPayment, 0),
		},
	}
	useCase, err := NewRunway(source, fixedClock{now: time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := useCase.Run(context.Background(), RunwayParams{
		MonthStartDay: 1,
		BaseCurrency:  cur(t, "EUR"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Liquidity.Minor() != 240_000 {
		t.Fatalf("liquidity = %d", got.Liquidity.Minor())
	}
	if got.ReferenceMonthlySpending.Minor() != 100_000 {
		t.Fatalf("reference spending = %d", got.ReferenceMonthlySpending.Minor())
	}
	if got.Months != 2 || got.Days != 12 {
		t.Fatalf("duration = %d months %d days", got.Months, got.Days)
	}
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time { return clock.now }

type fakeSource struct {
	accounts     []domain.Account
	transactions []domain.Transaction
	balances     map[domain.AccountID]domain.Money
	rates        map[string][]fxPoint
}

type fxPoint struct {
	date domain.Date
	rate domain.Rate
}

func (source *fakeSource) ListAccounts(_ context.Context, filter AccountFilter) ([]domain.Account, error) {
	accountIDs := toSet(filter.AccountIDs)
	accounts := make([]domain.Account, 0, len(source.accounts))
	for _, account := range source.accounts {
		if !filter.IncludeArchived && account.Archived {
			continue
		}
		if len(accountIDs) > 0 && !accountIDs[account.ID] {
			continue
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (source *fakeSource) ListTransactions(_ context.Context, filter TransactionFilter) ([]domain.Transaction, error) {
	accountIDs := toSet(filter.AccountIDs)
	txs := make([]domain.Transaction, 0, len(source.transactions))
	for _, tx := range source.transactions {
		if !filter.Period.Contains(tx.Date) {
			continue
		}
		if len(accountIDs) > 0 && !accountIDs[tx.AccountID] {
			continue
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (source *fakeSource) BalanceAt(_ context.Context, accountID domain.AccountID, _ domain.Date) (domain.Money, error) {
	balance, ok := source.balances[accountID]
	if !ok {
		return domain.Money{}, fmt.Errorf("balance not found for account %d", accountID)
	}
	return balance, nil
}

func (source *fakeSource) FXRateOnOrBefore(_ context.Context, from domain.Currency, to domain.Currency, at domain.Date) (domain.Rate, error) {
	key := from.Code() + "->" + to.Code()
	points := source.rates[key]
	var (
		best    domain.Rate
		bestSet bool
		bestDay domain.Date
	)
	for _, point := range points {
		if point.date.After(at) {
			continue
		}
		if !bestSet || point.date.After(bestDay) {
			best = point.rate
			bestDay = point.date
			bestSet = true
		}
	}
	if !bestSet {
		return 0, errors.New("rate not found")
	}
	return best, nil
}

func toSet(ids []domain.AccountID) map[domain.AccountID]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[domain.AccountID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func cur(t *testing.T, code string) domain.Currency {
	t.Helper()
	currency, err := domain.ParseCurrency(code)
	if err != nil {
		t.Fatalf("ParseCurrency(%q): %v", code, err)
	}
	return currency
}

func money(t *testing.T, minor int64, code string) domain.Money {
	t.Helper()
	value, err := domain.NewMoney(minor, cur(t, code))
	if err != nil {
		t.Fatalf("NewMoney(%d,%s): %v", minor, code, err)
	}
	return value
}

func date(t *testing.T, y, m, d int) domain.Date {
	t.Helper()
	day, err := domain.NewDate(y, m, d)
	if err != nil {
		t.Fatalf("NewDate(%d,%d,%d): %v", y, m, d, err)
	}
	return day
}

func period(t *testing.T, from, to domain.Date) domain.Period {
	t.Helper()
	p, err := domain.NewPeriod(from, to)
	if err != nil {
		t.Fatalf("NewPeriod(%v,%v): %v", from, to, err)
	}
	return p
}

func rate(t *testing.T, scaled int64) domain.Rate {
	t.Helper()
	r, err := domain.NewRate(scaled)
	if err != nil {
		t.Fatalf("NewRate(%d): %v", scaled, err)
	}
	return r
}

func account(t *testing.T, id domain.AccountID, name string, kind domain.AccountKind, code string, archived bool) domain.Account {
	t.Helper()
	a, err := domain.NewAccount(domain.Account{
		ID:             id,
		Name:           name,
		Kind:           kind,
		Currency:       cur(t, code),
		OpeningBalance: money(t, 0, code),
		OpenedOn:       date(t, 2025, 1, 1),
		Archived:       archived,
	})
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	return a
}

func makeTx(t *testing.T, accountID domain.AccountID, d domain.Date, minor int64, code string, kind domain.TxKind, categoryID domain.CategoryID) domain.Transaction {
	t.Helper()
	tx, err := domain.NewTransaction(domain.Transaction{
		AccountID:  accountID,
		Date:       d,
		Amount:     money(t, minor, code),
		Status:     domain.StatusBooked,
		Kind:       kind,
		CategoryID: categoryID,
	})
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}
	return tx
}
