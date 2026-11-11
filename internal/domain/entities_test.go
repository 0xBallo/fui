package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestEnums(t *testing.T) {
	for _, k := range []AccountKind{AccountCash, AccountChecking, AccountCreditCard, AccountSavings, AccountInvestment, AccountLoan} {
		if got, err := ParseAccountKind(" " + strings.ToUpper(string(k))); err != nil || got != k || !k.Valid() {
			t.Errorf("account kind %q: %v, %v", k, got, err)
		}
	}
	for _, k := range []TxKind{KindPayment, KindTransfer, KindExchange, KindFee, KindInterest, KindRefund, KindInvestment, KindOther} {
		if got, err := ParseTxKind(string(k)); err != nil || got != k {
			t.Errorf("tx kind %q: %v, %v", k, got, err)
		}
	}
	for _, s := range []TxStatus{StatusPending, StatusBooked, StatusReconciled} {
		if got, err := ParseTxStatus(string(s)); err != nil || got != s {
			t.Errorf("status %q: %v, %v", s, got, err)
		}
	}
	for _, k := range []CategoryKind{CategoryIncome, CategoryExpense} {
		if got, err := ParseCategoryKind(string(k)); err != nil || got != k {
			t.Errorf("category kind %q: %v, %v", k, got, err)
		}
	}
	if _, err := ParseAccountKind("bank"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("invalid account kind err = %v", err)
	}
	if _, err := ParseTxKind(""); !errors.Is(err, ErrMissingField) {
		t.Errorf("empty tx kind err = %v", err)
	}
	if _, err := ParseTxStatus("done"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("invalid status err = %v", err)
	}
	if _, err := ParseCategoryKind("both"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("invalid category kind err = %v", err)
	}
	if AccountKind("").Valid() || TxKind("x").Valid() || TxStatus("").Valid() || CategoryKind("").Valid() {
		t.Error("invalid enum reported valid")
	}
}

func validAccount(t tb) Account {
	return Account{
		Name: "  Fineco ", Kind: AccountChecking, Currency: cur(t, "EUR"),
		OpeningBalance: money(t, 10000, "EUR"), OpenedOn: date(t, 2026, 1, 1),
	}
}

func TestNewAccount(t *testing.T) {
	got, err := NewAccount(validAccount(t))
	if err != nil || got.Name != "Fineco" {
		t.Fatalf("NewAccount = %+v, %v", got, err)
	}
	withIBAN := validAccount(t)
	withIBAN.IBAN = "de89 3704 0044 0532 0130 00"
	got, err = NewAccount(withIBAN)
	if err != nil || got.IBAN != "DE89370400440532013000" {
		t.Fatalf("NewAccount with iban = %+v, %v", got, err)
	}
	tests := []struct {
		name    string
		mutate  func(*Account)
		wantErr error
	}{
		{"blank name", func(a *Account) { a.Name = "  " }, ErrMissingField},
		{"no kind", func(a *Account) { a.Kind = "" }, ErrMissingField},
		{"bad kind", func(a *Account) { a.Kind = "bank" }, ErrInvalidValue},
		{"no currency", func(a *Account) { a.Currency = Currency{} }, ErrMissingField},
		{"opening in other currency", func(a *Account) { a.OpeningBalance = money(t, 1, "USD") }, ErrCurrencyMismatch},
		{"no opening date", func(a *Account) { a.OpenedOn = Date{} }, ErrMissingField},
		{"bad iban", func(a *Account) { a.IBAN = "not-an-iban" }, ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := validAccount(t)
			tt.mutate(&a)
			if _, err := NewAccount(a); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func validTx(t tb) Transaction {
	return Transaction{
		AccountID: 1, Date: date(t, 2026, 11, 11), Amount: money(t, -450, "EUR"),
		Description: " Coffee ", Status: StatusBooked, Kind: KindPayment, ExternalID: " abc ",
	}
}

func TestNewTransaction(t *testing.T) {
	got, err := NewTransaction(validTx(t))
	if err != nil || got.Description != "Coffee" || got.ExternalID != "abc" {
		t.Fatalf("NewTransaction = %+v, %v", got, err)
	}
	tests := []struct {
		name    string
		mutate  func(*Transaction)
		wantErr error
	}{
		{"no account", func(x *Transaction) { x.AccountID = 0 }, ErrMissingField},
		{"no date", func(x *Transaction) { x.Date = Date{} }, ErrMissingField},
		{"no amount", func(x *Transaction) { x.Amount = Money{} }, ErrMissingField},
		{"no status", func(x *Transaction) { x.Status = "" }, ErrMissingField},
		{"bad status", func(x *Transaction) { x.Status = "done" }, ErrInvalidValue},
		{"no kind", func(x *Transaction) { x.Kind = "" }, ErrMissingField},
		{"bad kind", func(x *Transaction) { x.Kind = "gift" }, ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := validTx(t)
			tt.mutate(&x)
			if _, err := NewTransaction(x); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewTransferGroup(t *testing.T) {
	leg := func(acc AccountID, minor int64, code string) TransferLeg {
		return TransferLeg{AccountID: acc, Amount: money(t, minor, code)}
	}
	tests := []struct {
		name    string
		g       TransferGroup
		wantErr error
	}{
		{"no fee", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 10000, "EUR")}, nil},
		{"zero fee explicit", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 10000, "EUR"), Fee: money(t, 0, "EUR")}, nil},
		{"with fee", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 9800, "EUR"), Fee: money(t, 200, "EUR")}, nil},
		{"cross currency", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 10850, "USD")}, nil},
		{"unbalanced", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 9900, "EUR")}, ErrInvalidValue},
		{"unbalanced with fee", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 9900, "EUR"), Fee: money(t, 200, "EUR")}, ErrInvalidValue},
		{"negative fee", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 10200, "EUR"), Fee: money(t, -200, "EUR")}, ErrInvalidValue},
		{"fee in other currency", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 9800, "EUR"), Fee: money(t, 200, "USD")}, ErrCurrencyMismatch},
		{"fee on cross currency", TransferGroup{Out: leg(1, -10000, "EUR"), In: leg(2, 10850, "USD"), Fee: money(t, 1, "EUR")}, ErrInvalidValue},
		{"same account", TransferGroup{Out: leg(1, -100, "EUR"), In: leg(1, 100, "EUR")}, ErrInvalidValue},
		{"missing account", TransferGroup{Out: leg(1, -100, "EUR"), In: leg(0, 100, "EUR")}, ErrMissingField},
		{"wrong signs", TransferGroup{Out: leg(1, 100, "EUR"), In: leg(2, -100, "EUR")}, ErrInvalidValue},
		{"zero out", TransferGroup{Out: leg(1, 0, "EUR"), In: leg(2, 100, "EUR")}, ErrInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTransferGroup(tt.g)
			if tt.wantErr == nil && err != nil || tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCategory(t *testing.T) {
	if c, err := NewCategory(" Home ", CategoryExpense); err != nil || c.Name != "Home" || c.Kind != CategoryExpense {
		t.Errorf("NewCategory = %+v, %v", c, err)
	}
	if _, err := NewCategory("", CategoryExpense); !errors.Is(err, ErrMissingField) {
		t.Errorf("blank err = %v", err)
	}
	if _, err := NewCategory("x", ""); !errors.Is(err, ErrMissingField) {
		t.Errorf("no kind err = %v", err)
	}
	if _, err := NewCategory("x", "both"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("bad kind err = %v", err)
	}
}

func TestDefaultCategories(t *testing.T) {
	cats := DefaultCategories()
	if len(cats) == 0 {
		t.Fatal("no default categories")
	}
	seen := map[string]bool{}
	var income, expense int
	for _, c := range cats {
		if _, err := NewCategory(c.Name, c.Kind); err != nil {
			t.Errorf("invalid default %+v: %v", c, err)
		}
		key := strings.ToLower(c.Name)
		if seen[key] {
			t.Errorf("duplicate default %q", c.Name)
		}
		seen[key] = true
		if c.Kind == CategoryIncome {
			income++
		} else {
			expense++
		}
	}
	if income == 0 || expense == 0 {
		t.Errorf("income=%d expense=%d", income, expense)
	}
	cats[0].Name = "mutated"
	if DefaultCategories()[0].Name == "mutated" {
		t.Error("DefaultCategories shares state between calls")
	}
}

func TestNewFXRate(t *testing.T) {
	usd, d, r := cur(t, "USD"), date(t, 2026, 11, 11), rate(t, 1_085_000_000)
	if got, err := NewFXRate(usd, d, r); err != nil || got.Rate != r || got.Currency != usd || got.Date != d {
		t.Errorf("NewFXRate = %+v, %v", got, err)
	}
	if _, err := NewFXRate(Currency{}, d, r); !errors.Is(err, ErrMissingField) {
		t.Errorf("no currency err = %v", err)
	}
	if _, err := NewFXRate(usd, Date{}, r); !errors.Is(err, ErrMissingField) {
		t.Errorf("no date err = %v", err)
	}
	if _, err := NewFXRate(usd, d, 0); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("zero rate err = %v", err)
	}
}
