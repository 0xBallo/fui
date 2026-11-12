package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func bdate(t tb, y, m, d int) Date {
	t.Helper()
	v, err := NewDate(y, m, d)
	if err != nil {
		t.Fatalf("NewDate: %v", err)
	}
	return v
}

func testBudget(t tb, mode BudgetMode, amount int64) Budget {
	t.Helper()
	b, err := NewBudget(Budget{
		ID: 1, Name: "Food", Mode: mode, Amount: money(t, amount, "EUR"),
		Targets: []BudgetTarget{{CategoryID: 10}}, Carry: true,
	})
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	return b
}

func spend(t tb, d Date, minor int64, status TxStatus) BudgetRow {
	return BudgetRow{Date: d, Amount: money(t, -minor, "EUR"), Kind: KindPayment, Status: status, CategoryID: 10}
}

func TestNewBudgetErrors(t *testing.T) {
	ok := Budget{Name: "x", Mode: BudgetCap, Amount: money(t, 100, "EUR"), Targets: []BudgetTarget{{CategoryID: 1}}}
	tests := map[string]func(*Budget){
		"name":         func(b *Budget) { b.Name = " " },
		"mode":         func(b *Budget) { b.Mode = "x" },
		"no targets":   func(b *Budget) { b.Targets = nil },
		"both targets": func(b *Budget) { b.Targets = []BudgetTarget{{CategoryID: 1, TagID: 1}} },
		"no target":    func(b *Budget) { b.Targets = []BudgetTarget{{}} },
		"no amount":    func(b *Budget) { b.Amount = Money{} },
		"negative":     func(b *Budget) { b.Amount = money(t, -1, "EUR") },
		"bad month": func(b *Budget) {
			b.Overrides = map[MonthKey]Money{{2026, 13}: money(t, 1, "EUR")}
		},
		"override cur": func(b *Budget) {
			b.Overrides = map[MonthKey]Money{{2026, 1}: money(t, 1, "USD")}
		},
		"override neg": func(b *Budget) {
			b.Overrides = map[MonthKey]Money{{2026, 1}: money(t, -1, "EUR")}
		},
		"thresholds": func(b *Budget) { b.Thresholds = BudgetThresholds{Warn: 90, Over: 80} },
	}
	for name, mut := range tests {
		t.Run(name, func(t *testing.T) {
			b := ok
			mut(&b)
			if _, err := NewBudget(b); err == nil {
				t.Fatal("want error")
			}
		})
	}
	got, err := NewBudget(ok)
	if err != nil || got.Thresholds != DefaultBudgetThresholds {
		t.Fatalf("got %+v, %v", got.Thresholds, err)
	}
	if _, err := ParseBudgetMode("envelope"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBudgetMode("nope"); err == nil {
		t.Fatal("want error")
	}
}

func TestValidateBudgets(t *testing.T) {
	a := Budget{Name: "a", Targets: []BudgetTarget{{CategoryID: 1}, {TagID: 5}}}
	b := Budget{Name: "b", Targets: []BudgetTarget{{TagID: 5}}}
	if err := ValidateBudgets([]Budget{a, b}); err != nil {
		t.Fatalf("tag overlap must be allowed: %v", err)
	}
	c := Budget{Name: "c", Targets: []BudgetTarget{{CategoryID: 1}}}
	if err := ValidateBudgets([]Budget{a, c}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("got %v", err)
	}
	if !b.TagOnly() || a.TagOnly() {
		t.Fatal("TagOnly")
	}
}

func TestBudgetPeriodsAndOverride(t *testing.T) {
	ps, err := BudgetPeriods(bdate(t, 2026, 1, 20), 3, 15)
	if err != nil {
		t.Fatal(err)
	}
	if ps[0].From().String() != "2026-01-15" || ps[1].From().String() != "2026-02-15" || ps[2].To().String() != "2026-04-14" {
		t.Fatalf("periods %v", ps)
	}
	if _, err := BudgetPeriods(bdate(t, 2026, 1, 1), 1, 0); err == nil {
		t.Fatal("want error")
	}
	b := testBudget(t, BudgetCap, 100)
	b.Overrides = map[MonthKey]Money{MonthKeyOf(ps[1]): money(t, 500, "EUR")}
	if b.AmountFor(MonthKeyOf(ps[0])).Minor() != 100 || b.AmountFor(MonthKeyOf(ps[1])).Minor() != 500 {
		t.Fatal("override")
	}
}

func TestCapStatuses(t *testing.T) {
	b := testBudget(t, BudgetCap, 10000)
	ps, _ := BudgetPeriods(bdate(t, 2026, 1, 1), 3, 1)
	rows := []BudgetRow{
		spend(t, bdate(t, 2026, 1, 5), 8500, StatusBooked),  // 85% -> warning
		spend(t, bdate(t, 2026, 1, 6), 3000, StatusPending), // planned only
		{Date: bdate(t, 2026, 1, 7), Amount: money(t, 500, "EUR"), Kind: KindRefund, Status: StatusBooked, CategoryID: 10},
		{Date: bdate(t, 2026, 1, 8), Amount: money(t, -9999, "EUR"), Kind: KindTransfer, Status: StatusBooked, CategoryID: 10},
		{Date: bdate(t, 2026, 1, 8), Amount: money(t, -9999, "EUR"), Kind: KindPayment, Status: StatusBooked, CategoryID: 10, LoanPrincipal: true},
		spend(t, bdate(t, 2026, 2, 3), 15000, StatusBooked), // overspend 50 over avail 120 -> over
		{Date: bdate(t, 2026, 2, 4), Amount: money(t, -1, "EUR"), Kind: KindPayment, Status: StatusBooked, CategoryID: 99},
	}
	got, err := CapStatuses(b, ps, rows, Money{})
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		spent, planned, remaining int64
		pct                       int
		state                     BudgetState
	}
	wants := []want{{8000, 3000, 2000, 80, BudgetWarning}, {15000, 0, -3000, 125, BudgetOver}, {0, 0, 7000, 0, BudgetOK}} // month 3: available = 10000 - 3000
	for i, w := range wants {
		g := got[i]
		if g.Spent.Minor() != w.spent || g.Planned.Minor() != w.planned || g.Remaining.Minor() != w.remaining ||
			g.Percent != w.pct || g.State != w.state {
			t.Errorf("month %d: %+v", i, g)
		}
	}
	// no carry: each month independent.
	b.Carry = false
	got, _ = CapStatuses(b, ps, rows, Money{})
	if got[1].Available.Minor() != 10000 || got[2].CarryIn.Minor() != 0 {
		t.Fatalf("no-carry %+v", got[1])
	}
	// tag target + opening carry + currency error
	tb := b
	tb.Targets = []BudgetTarget{{TagID: 3}}
	tagRow := spend(t, bdate(t, 2026, 1, 9), 100, StatusBooked)
	tagRow.TagIDs = []TagID{3}
	got, _ = CapStatuses(tb, ps[:1], []BudgetRow{tagRow}, money(t, 50, "EUR"))
	if got[0].Available.Minor() != 10050 || got[0].Spent.Minor() != 100 {
		t.Fatalf("tag %+v", got[0])
	}
	bad := BudgetRow{Date: bdate(t, 2026, 1, 9), Amount: money(t, -1, "USD"), Kind: KindPayment, CategoryID: 10, Status: StatusBooked}
	if _, err := CapStatuses(b, ps, []BudgetRow{bad}, Money{}); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestPercentEdges(t *testing.T) {
	z, five := money(t, 0, "EUR"), money(t, 5, "EUR")
	if p := percent(CapStatus{Available: z, Spent: five}); p != 100 {
		t.Fatal(p)
	}
	if p := percent(CapStatus{Available: z, Spent: z}); p != 0 {
		t.Fatal(p)
	}
	if p := percent(CapStatus{Available: money(t, -5, "EUR"), Spent: z}); p != 100 {
		t.Fatal(p)
	}
	if p := percent(CapStatus{Available: money(t, 1, "EUR"), Spent: money(t, 1<<62, "EUR")}); p != 1<<31 {
		t.Fatal(p)
	}
	if s := capState(CapStatus{Remaining: five, Percent: 80}, BudgetThresholds{}); s != BudgetWarning {
		t.Fatal(s)
	}
}

func TestEnvelopeLedgerAndToAssign(t *testing.T) {
	b := testBudget(t, BudgetEnvelope, 0)
	ps, _ := BudgetPeriods(bdate(t, 2026, 1, 1), 2, 1)
	eur := func(m int64) Money { return money(t, m, "EUR") }
	moves := []BudgetMove{
		{Date: bdate(t, 2026, 1, 1), From: 0, To: 1, Amount: eur(10000)},
		{Date: bdate(t, 2026, 1, 2), From: 2, To: 1, Amount: eur(1000)},
		{Date: bdate(t, 2026, 1, 3), From: 1, To: 3, Amount: eur(500)},
		{Date: bdate(t, 2026, 2, 1), From: 1, To: 0, Amount: eur(200)},
		{Date: bdate(t, 2026, 2, 1), From: 7, To: 8, Amount: eur(9)}, // unrelated
	}
	rows := []BudgetRow{spend(t, bdate(t, 2026, 1, 10), 12000, StatusBooked), spend(t, bdate(t, 2026, 2, 10), 100, StatusPending)}
	got, err := EnvelopeLedger(b, ps, rows, moves, Money{})
	if err != nil {
		t.Fatal(err)
	}
	// Jan: 0+10000+1000-500-12000 = -1500 (overspend); Feb: -1500-200 = -1700.
	if got[0].Balance.Minor() != -1500 || got[0].Overspend.Minor() != 1500 ||
		got[1].CarryIn.Minor() != -1500 || got[1].Balance.Minor() != -1700 || got[1].Planned.Minor() != 100 {
		t.Fatalf("%+v", got)
	}
	if got[0].MovesIn.Minor() != 1000 || got[0].MovesOut.Minor() != 500 || got[1].Assigned.Minor() != -200 {
		t.Fatalf("moves %+v", got)
	}
	if _, err := EnvelopeLedger(b, ps, nil, []BudgetMove{{Date: bdate(t, 2026, 1, 1)}}, eur(5)); err == nil {
		t.Fatal("want invalid move error")
	}

	u, err := ToAssign(cur(t, "EUR"), []AccountBalance{
		{AccountChecking, eur(50000)}, {AccountCreditCard, eur(-9000)}, {AccountInvestment, eur(99999)}, {AccountCash, eur(1000)},
	}, []EnvelopeBalance{{Balance: eur(20000)}, {Balance: eur(-1500)}, {TagOnly: true, Balance: eur(777)}})
	if err != nil || u.Liquidity.Minor() != 51000 || u.Envelopes.Minor() != 18500 || u.ToAssign.Minor() != 32500 {
		t.Fatalf("%+v %v", u, err)
	}
	if _, err := ToAssign(cur(t, "EUR"), []AccountBalance{{AccountCash, money(t, 1, "USD")}}, nil); err == nil {
		t.Fatal("want error")
	}
	if _, err := ToAssign(cur(t, "EUR"), nil, []EnvelopeBalance{{Balance: money(t, 1, "USD")}}); err == nil {
		t.Fatal("want error")
	}
}

func TestMoveValidate(t *testing.T) {
	d, m := bdate(t, 2026, 1, 1), money(t, 1, "EUR")
	for name, mv := range map[string]BudgetMove{
		"date": {Amount: m, To: 1}, "amount": {Date: d, To: 1}, "sign": {Date: d, To: 1, Amount: money(t, 0, "EUR")},
		"same": {Date: d, From: 1, To: 1, Amount: m},
	} {
		if mv.Validate() == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestUncoveredSpending(t *testing.T) {
	b := testBudget(t, BudgetEnvelope, 0)
	ps, _ := BudgetPeriods(bdate(t, 2026, 1, 1), 1, 1)
	other := spend(t, bdate(t, 2026, 1, 3), 700, StatusBooked)
	other.CategoryID = 99
	rows := []BudgetRow{
		spend(t, bdate(t, 2026, 1, 2), 500, StatusBooked), other,
		{Date: bdate(t, 2026, 1, 4), Amount: money(t, 900, "EUR"), Kind: KindPayment, Status: StatusBooked, CategoryID: 99},
	}
	got, err := UncoveredSpending([]Budget{b}, ps[0], rows, cur(t, "EUR"))
	if err != nil || got.Minor() != 700 {
		t.Fatalf("%v %v", got, err)
	}
	rows[1].Amount = money(t, -1, "USD")
	if _, err := UncoveredSpending(nil, ps[0], rows[1:2], cur(t, "EUR")); err == nil {
		t.Fatal("want error")
	}
}

// Envelopes + Da assegnare == included liquidity, and moving money between envelopes or the pool
// never changes the total once the pool is accounted for.
func TestToAssignInvariantRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eur := cur(t, "EUR")
		kinds := []AccountKind{AccountCash, AccountChecking, AccountSavings, AccountCreditCard, AccountLoan, AccountInvestment}
		accs := rapid.SliceOfN(rapid.Custom(func(t *rapid.T) AccountBalance {
			return AccountBalance{rapid.SampledFrom(kinds).Draw(t, "kind"), money(t, rapid.Int64Range(-1e9, 1e9).Draw(t, "bal"), "EUR")}
		}), 0, 8).Draw(t, "accounts")
		envs := rapid.SliceOfN(rapid.Custom(func(t *rapid.T) EnvelopeBalance {
			return EnvelopeBalance{rapid.Bool().Draw(t, "tag"), money(t, rapid.Int64Range(-1e9, 1e9).Draw(t, "e"), "EUR")}
		}), 0, 8).Draw(t, "envs")
		u, err := ToAssign(eur, accs, envs)
		if err != nil {
			t.Fatal(err)
		}
		sum, _ := u.Envelopes.Add(u.ToAssign)
		var want int64
		for _, a := range accs {
			if a.Kind.IsLiquidity() {
				want += a.Balance.Minor()
			}
		}
		if sum.Minor() != want || u.Liquidity.Minor() != want {
			t.Fatalf("envelopes+toassign=%d want %d", sum.Minor(), want)
		}
	})
}

// Carry-over conserves money: final remaining = opening + sum(amounts) - sum(spent).
func TestCarryConservationRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := testBudget(t, BudgetCap, rapid.Int64Range(0, 1e6).Draw(t, "amount"))
		n := rapid.IntRange(1, 12).Draw(t, "months")
		ps, err := BudgetPeriods(bdate(t, 2026, 1, 1), n, rapid.IntRange(1, 28).Draw(t, "start"))
		if err != nil {
			t.Fatal(err)
		}
		var rows []BudgetRow
		var spentTotal int64
		for range rapid.IntRange(0, 20).Draw(t, "rows") {
			p := ps[rapid.IntRange(0, n-1).Draw(t, "p")]
			amt := rapid.Int64Range(0, 1e6).Draw(t, "spend")
			rows = append(rows, spend(t, p.From(), amt, StatusBooked))
			spentTotal += amt
		}
		opening := rapid.Int64Range(-1e6, 1e6).Draw(t, "opening")
		got, err := CapStatuses(b, ps, rows, money(t, opening, "EUR"))
		if err != nil {
			t.Fatal(err)
		}
		want := opening + int64(n)*b.Amount.Minor() - spentTotal
		if last := got[n-1].Remaining.Minor(); last != want {
			t.Fatalf("remaining %d want %d", last, want)
		}
		for i := 1; i < n; i++ {
			if got[i].CarryIn != got[i-1].Remaining {
				t.Fatalf("carry break at %d", i)
			}
		}
	})
}
