package domain

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

func mustDate(t tb, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func qty(t tb, s string, scale int) Quantity {
	t.Helper()
	q, err := ParseQuantity(s, scale)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func TestQuantityParseString(t *testing.T) {
	tests := []struct {
		in    string
		scale int
		want  string
		err   error
	}{
		{"12", 0, "12", nil},
		{"-0.5", 1, "-0.5", nil},
		{"+1.5", 2, "1.50", nil},
		{"0.00000001", 8, "0.00000001", nil},
		{"-0.00000123", 8, "-0.00000123", nil},
		{".5", 1, "0.5", nil},
		{"1.5", 0, "", ErrInvalidAmount},
		{"abc", 2, "", ErrInvalidAmount},
		{"", 2, "", ErrInvalidAmount},
		{"1.2.3", 2, "", ErrInvalidAmount},
		{"1", 9, "", ErrInvalidValue},
		{"99999999999999999999", 0, "", ErrOverflow},
	}
	for _, tc := range tests {
		q, err := ParseQuantity(tc.in, tc.scale)
		if tc.err != nil {
			if !errors.Is(err, tc.err) {
				t.Errorf("ParseQuantity(%q,%d) err = %v, want %v", tc.in, tc.scale, err, tc.err)
			}
			continue
		}
		if err != nil || q.String() != tc.want {
			t.Errorf("ParseQuantity(%q,%d) = %v, %v; want %s", tc.in, tc.scale, q, err, tc.want)
		}
	}
}

func TestQuantityCryptoRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64().Draw(t, "units")
		q, err := NewQuantity(n, 8)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseQuantity(q.String(), 8)
		if err != nil || back != q {
			t.Fatalf("round trip %d: %v %v", n, back, err)
		}
	})
}

func TestQuantityArithmetic(t *testing.T) {
	a, b := qty(t, "1.5", 2), qty(t, "0.25", 2)
	sum, err := a.Add(b)
	if err != nil || sum.String() != "1.75" {
		t.Fatalf("Add = %v, %v", sum, err)
	}
	diff, err := a.Sub(b)
	if err != nil || diff.String() != "1.25" {
		t.Fatalf("Sub = %v, %v", diff, err)
	}
	neg, err := a.Neg()
	if err != nil || neg.String() != "-1.50" || neg.Sign() != -1 || a.Sign() != 1 || !qty(t, "0", 2).IsZero() {
		t.Fatalf("Neg = %v, %v", neg, err)
	}
	if c, err := a.Cmp(b); err != nil || c != 1 {
		t.Fatalf("Cmp = %d, %v", c, err)
	}
	if c, _ := b.Cmp(a); c != -1 {
		t.Fatalf("Cmp = %d", c)
	}
	if c, _ := a.Cmp(a); c != 0 {
		t.Fatalf("Cmp = %d", c)
	}
	other := qty(t, "1", 0)
	if _, err := a.Add(other); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Add mismatch err = %v", err)
	}
	if _, err := a.Sub(other); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Sub mismatch err = %v", err)
	}
	if _, err := a.Cmp(other); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Cmp mismatch err = %v", err)
	}
	maxQ, _ := NewQuantity(1<<63-1, 0)
	minQ, _ := NewQuantity(-1<<63, 0)
	one := qty(t, "1", 0)
	if _, err := maxQ.Add(one); !errors.Is(err, ErrOverflow) {
		t.Errorf("Add overflow err = %v", err)
	}
	if _, err := minQ.Sub(one); !errors.Is(err, ErrOverflow) {
		t.Errorf("Sub overflow err = %v", err)
	}
	if _, err := minQ.Neg(); !errors.Is(err, ErrOverflow) {
		t.Errorf("Neg overflow err = %v", err)
	}
	if _, err := NewQuantity(1, -1); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("NewQuantity scale err = %v", err)
	}
	if a.Units() != 150 || a.Scale() != 2 {
		t.Errorf("accessors: %d %d", a.Units(), a.Scale())
	}
}

func TestQuantityValueRoundsOnce(t *testing.T) {
	// 0.333 × 10.00 = 3.33; 1/3 unit at scale 8 × 1.00 EUR = 0.33333333 → 0.33
	tests := []struct {
		q     string
		scale int
		price int64
		want  int64
	}{
		{"0.333", 3, 1000, 333},
		{"0.33333333", 8, 100, 33},
		{"0.00000005", 8, 100, 0},
		{"0.005", 3, 100, 1},
		{"-0.005", 3, 100, -1},
		{"3", 0, 1999, 5997},
	}
	for _, tc := range tests {
		got, err := qty(t, tc.q, tc.scale).Value(money(t, tc.price, "EUR"))
		if err != nil || got.Minor() != tc.want {
			t.Errorf("%s × %d = %v, %v; want %d", tc.q, tc.price, got, err, tc.want)
		}
	}
	big, _ := NewQuantity(1<<62, 0)
	if _, err := big.Value(money(t, 1<<40, "EUR")); !errors.Is(err, ErrOverflow) {
		t.Errorf("overflow err = %v", err)
	}
}

func TestNewSecurity(t *testing.T) {
	eur := cur(t, "EUR")
	ok := Security{Name: " Vanguard ", Ticker: " vwce ", ISIN: "ie00bk5bqt80", Kind: SecurityETF, Currency: eur, QuantityScale: 3}
	got, err := NewSecurity(ok)
	if err != nil || got.Name != "Vanguard" || got.Ticker != "VWCE" || got.ISIN != "IE00BK5BQT80" {
		t.Fatalf("NewSecurity = %+v, %v", got, err)
	}
	bad := map[string]struct {
		mod func(*Security)
		err error
	}{
		"no name":     {func(s *Security) { s.Name = " " }, ErrMissingField},
		"no id":       {func(s *Security) { s.Ticker, s.ISIN = "", "" }, ErrMissingField},
		"bad isin":    {func(s *Security) { s.ISIN = "IE00BK5BQT8" }, ErrInvalidValue},
		"isin prefix": {func(s *Security) { s.ISIN = "1E00BK5BQT80" }, ErrInvalidValue},
		"isin body":   {func(s *Security) { s.ISIN = "IE00BK5B-T80" }, ErrInvalidValue},
		"isin check":  {func(s *Security) { s.ISIN = "IE00BK5BQT8X" }, ErrInvalidValue},
		"no kind":     {func(s *Security) { s.Kind = "" }, ErrMissingField},
		"bad kind":    {func(s *Security) { s.Kind = "nft" }, ErrInvalidValue},
		"no currency": {func(s *Security) { s.Currency = Currency{} }, ErrMissingField},
		"bad scale":   {func(s *Security) { s.QuantityScale = 9 }, ErrInvalidValue},
	}
	for name, tc := range bad {
		s := ok
		tc.mod(&s)
		if _, err := NewSecurity(s); !errors.Is(err, tc.err) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.err)
		}
	}
	if k, err := ParseSecurityKind(" Crypto "); err != nil || k != SecurityCrypto {
		t.Errorf("ParseSecurityKind = %v, %v", k, err)
	}
	if k, err := ParseSecurityOpKind("COUPON"); err != nil || k != OpCoupon {
		t.Errorf("ParseSecurityOpKind = %v, %v", k, err)
	}
	if _, err := ParseSecurityOpKind("x"); err == nil {
		t.Error("want error for bad op kind")
	}
}

func TestNewSecurityOp(t *testing.T) {
	ok := SecurityOp{
		Date: mustDate(t, "2026-01-05"), Account: 1, Security: 2, Kind: OpBuy,
		Quantity: qty(t, "2", 0), UnitPrice: money(t, 1000, "EUR"),
	}
	got, err := NewSecurityOp(ok)
	if err != nil || got.Fees.Minor() != 0 || got.Fees.Currency().Code() != "EUR" {
		t.Fatalf("NewSecurityOp = %+v, %v", got, err)
	}
	bad := map[string]struct {
		mod func(*SecurityOp)
		err error
	}{
		"kind":        {func(o *SecurityOp) { o.Kind = "" }, ErrMissingField},
		"date":        {func(o *SecurityOp) { o.Date = Date{} }, ErrMissingField},
		"account":     {func(o *SecurityOp) { o.Account = 0 }, ErrMissingField},
		"security":    {func(o *SecurityOp) { o.Security = 0 }, ErrMissingField},
		"zero qty":    {func(o *SecurityOp) { o.Quantity = qty(t, "0", 0) }, ErrInvalidValue},
		"no price":    {func(o *SecurityOp) { o.UnitPrice = Money{} }, ErrMissingField},
		"neg price":   {func(o *SecurityOp) { o.UnitPrice = money(t, -1, "EUR") }, ErrInvalidValue},
		"fees ccy":    {func(o *SecurityOp) { o.Fees = money(t, 1, "USD") }, ErrCurrencyMismatch},
		"neg fees":    {func(o *SecurityOp) { o.Fees = money(t, -1, "EUR") }, ErrInvalidValue},
		"unknown knd": {func(o *SecurityOp) { o.Kind = "swap" }, ErrInvalidValue},
	}
	for name, tc := range bad {
		o := ok
		tc.mod(&o)
		if _, err := NewSecurityOp(o); !errors.Is(err, tc.err) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.err)
		}
	}
}

func testSec(t tb, scale int) Security {
	return Security{ID: 7, Name: "X", Ticker: "X", Kind: SecurityStock, Currency: cur(t, "EUR"), QuantityScale: scale}
}

func op(t tb, kind SecurityOpKind, date, q string, scale int, price, fees int64) SecurityOp {
	return SecurityOp{
		Date: mustDate(t, date), Account: 1, Security: 7, Kind: kind,
		Quantity: qty(t, q, scale), UnitPrice: money(t, price, "EUR"), Fees: money(t, fees, "EUR"),
	}
}

func TestComputePosition(t *testing.T) {
	sec := testSec(t, 0)
	ops := []SecurityOp{
		op(t, OpBuy, "2026-01-02", "10", 0, 1000, 100),  // cost 10100
		op(t, OpBuy, "2026-02-02", "10", 0, 1200, 0),    // cost 22100, avg 1105
		op(t, OpSell, "2026-03-02", "5", 0, 1500, 50),   // proceeds 7450, removed 5525 → +1925
		op(t, OpDividend, "2026-03-10", "15", 0, 20, 0), // 300
		op(t, OpBuy, "2026-06-01", "1", 0, 1, 0),        // after asOf
	}
	prices := []SecurityPrice{
		{Security: 7, Date: mustDate(t, "2026-01-02"), Price: money(t, 1000, "EUR")},
		{Security: 7, Date: mustDate(t, "2026-03-15"), Price: money(t, 1400, "EUR")},
		{Security: 7, Date: mustDate(t, "2026-04-15"), Price: money(t, 9999, "EUR")}, // after asOf
		{Security: 8, Date: mustDate(t, "2026-03-16"), Price: money(t, 1, "EUR")},    // other security
	}
	got, err := ComputePosition(sec, ops, prices, mustDate(t, "2026-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	want := Position{
		Security: 7, AsOf: mustDate(t, "2026-03-31"), Quantity: qty(t, "15", 0),
		CostBasis: money(t, 16575, "EUR"), AvgCost: money(t, 1105, "EUR"),
		Realized: money(t, 1925, "EUR"), Income: money(t, 300, "EUR"),
		HasPrice: true, LastPrice: money(t, 1400, "EUR"), PriceDate: mustDate(t, "2026-03-15"),
		MarketValue: money(t, 21000, "EUR"), Unrealized: money(t, 4425, "EUR"),
	}
	if d := cmp.Diff(want, got, cmp.AllowUnexported(Quantity{}, Money{}, Currency{}, Date{})); d != "" {
		t.Errorf("position mismatch (-want +got):\n%s", d)
	}
}

func TestComputePositionNoPriceAndEmpty(t *testing.T) {
	sec := testSec(t, 8)
	got, err := ComputePosition(sec, nil, nil, mustDate(t, "2026-01-01"))
	if err != nil || got.HasPrice || !got.Quantity.IsZero() || got.AvgCost.Minor() != 0 || got.Quantity.Scale() != 8 {
		t.Fatalf("empty = %+v, %v", got, err)
	}
}

func TestComputePositionErrors(t *testing.T) {
	sec := testSec(t, 0)
	asOf := mustDate(t, "2026-12-31")
	t.Run("oversell", func(t *testing.T) {
		_, err := ComputePosition(sec, []SecurityOp{
			op(t, OpBuy, "2026-01-01", "1", 0, 100, 0), op(t, OpSell, "2026-01-02", "2", 0, 100, 0),
		}, nil, asOf)
		if !errors.Is(err, ErrInsufficientQuantity) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("same-day sell uses same-day buy", func(t *testing.T) {
		_, err := ComputePosition(sec, []SecurityOp{
			op(t, OpSell, "2026-01-01", "1", 0, 100, 0), op(t, OpBuy, "2026-01-01", "1", 0, 100, 0),
		}, nil, asOf)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("other security", func(t *testing.T) {
		o := op(t, OpBuy, "2026-01-01", "1", 0, 100, 0)
		o.Security = 9
		if _, err := ComputePosition(sec, []SecurityOp{o}, nil, asOf); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("op currency", func(t *testing.T) {
		o := op(t, OpBuy, "2026-01-01", "1", 0, 100, 0)
		o.UnitPrice = money(t, 100, "USD")
		if _, err := ComputePosition(sec, []SecurityOp{o}, nil, asOf); !errors.Is(err, ErrCurrencyMismatch) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("price currency", func(t *testing.T) {
		p := []SecurityPrice{{Security: 7, Date: mustDate(t, "2026-01-01"), Price: money(t, 1, "USD")}}
		if _, err := ComputePosition(sec, nil, p, asOf); !errors.Is(err, ErrCurrencyMismatch) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("scale mismatch", func(t *testing.T) {
		o := op(t, OpBuy, "2026-01-01", "1.5", 1, 100, 0)
		if _, err := ComputePosition(sec, []SecurityOp{o}, nil, asOf); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestLastPriceOnOrBefore(t *testing.T) {
	prices := []SecurityPrice{
		{Security: 1, Date: mustDate(t, "2026-01-02"), Price: money(t, 1, "EUR")},
		{Security: 1, Date: mustDate(t, "2026-01-02"), Price: money(t, 2, "EUR")},
	}
	got, ok := LastPriceOnOrBefore(prices, 1, mustDate(t, "2026-01-02"))
	if !ok || got.Price.Minor() != 2 {
		t.Errorf("got %v %v", got, ok)
	}
	if _, ok := LastPriceOnOrBefore(prices, 1, mustDate(t, "2026-01-01")); ok {
		t.Error("want no price")
	}
}

func TestPositionQuantityProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sec := testSec(t, 8)
		day := rapid.IntRange(1, 28).Draw(t, "day")
		date := Date{year: 2026, month: 3, day: day}
		buys := rapid.SliceOfN(rapid.Int64Range(1, 1_000_000_000_000), 1, 8).Draw(t, "buys")
		var total, sold int64
		var ops []SecurityOp
		mk := func(kind SecurityOpKind, u int64, price int64) SecurityOp {
			q, _ := NewQuantity(u, 8)
			return SecurityOp{
				Date: date, Account: 1, Security: 7, Kind: kind, Quantity: q,
				UnitPrice: money(t, price, "EUR"), Fees: money(t, 3, "EUR"),
			}
		}
		for _, b := range buys {
			total += b
			ops = append(ops, mk(OpBuy, b, rapid.Int64Range(0, 100000).Draw(t, "p")))
		}
		nSell := rapid.IntRange(0, 3).Draw(t, "nSell")
		for range nSell {
			s := rapid.Int64Range(1, max(1, (total-sold)/2)).Draw(t, "s")
			if sold+s > total {
				break
			}
			sold += s
			ops = append(ops, mk(OpSell, s, rapid.Int64Range(0, 100000).Draw(t, "sp")))
		}
		shuffled := rapid.Permutation(ops).Draw(t, "perm")
		a, err1 := ComputePosition(sec, ops, nil, date)
		b, err2 := ComputePosition(sec, shuffled, nil, date)
		if err1 != nil || err2 != nil {
			t.Fatalf("errors: %v %v", err1, err2)
		}
		if a.Quantity.Units() != total-sold {
			t.Fatalf("quantity = %d, want %d", a.Quantity.Units(), total-sold)
		}
		if diff := cmp.Diff(a, b, cmp.AllowUnexported(Quantity{}, Money{}, Currency{}, Date{})); diff != "" {
			t.Fatalf("order dependent:\n%s", diff)
		}
	})
}
