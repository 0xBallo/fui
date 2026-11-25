package domain

import (
	"errors"
	"math/big"
	"testing"

	"pgregory.net/rapid"
)

func TestNewPrice(t *testing.T) {
	eur, jpy := cur(t, "EUR"), cur(t, "JPY")
	tests := []struct {
		name  string
		minor int64
		scale int
		c     Currency
		want  string
		err   error
	}{
		{"eur at 2", 12345, 2, eur, "123.45", nil},
		{"btc sub-cent", 1234, 8, cur(t, "USD"), "0.00001234", nil},
		{"one unit at 8", 1, 8, eur, "0.00000001", nil},
		{"jpy at 0", 150, 0, jpy, "150", nil},
		{"jpy at 3", -1500, 3, jpy, "-1.500", nil},
		{"zero", 0, 4, eur, "0.0000", nil},
		{"below currency decimals", 1, 1, eur, "", ErrInvalidValue},
		{"above max", 1, 9, eur, "", ErrInvalidValue},
		{"negative scale", 1, -1, jpy, "", ErrInvalidValue},
		{"zero currency", 1, 2, Currency{}, "", ErrMissingField},
	}
	for _, tc := range tests {
		p, err := NewPrice(tc.minor, tc.scale, tc.c)
		if tc.err != nil {
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: err = %v, want %v", tc.name, err, tc.err)
			}
			continue
		}
		if err != nil || p.String() != tc.want {
			t.Errorf("%s: = %q, %v; want %q", tc.name, p, err, tc.want)
		}
	}
	p := priceOf(t, -5, 8, "EUR")
	if p.Minor() != -5 || p.Scale() != 8 || p.Currency().Code() != "EUR" || p.Sign() != -1 || p.IsZero() {
		t.Errorf("accessors: %+v", p)
	}
	if z := priceOf(t, 0, 2, "EUR"); z.Sign() != 0 || !z.IsZero() {
		t.Errorf("zero price: %+v", z)
	}
	if o := priceOf(t, 5, 2, "EUR"); o.Sign() != 1 {
		t.Errorf("positive sign: %d", o.Sign())
	}
}

func TestParsePrice(t *testing.T) {
	usd, jpy, eur := cur(t, "USD"), cur(t, "JPY"), cur(t, "EUR")
	tests := []struct {
		in    string
		c     Currency
		scale int
		loc   Locale
		want  string
		err   error
	}{
		{"0.00001234", usd, 8, LocaleENUS, "0.00001234", nil},
		{"0,00001234", usd, 8, LocaleITIT, "0.00001234", nil},
		{"1.234,5", eur, 8, LocaleITIT, "1234.50000000", nil},
		{"1,234.5", eur, 2, LocaleENUS, "1234.50", nil},
		{"+12", jpy, 0, LocaleENUS, "12", nil},
		{"-0.5", eur, 4, LocaleENUS, "-0.5000", nil},
		{"0.125", eur, 2, LocaleENUS, "0.13", nil},
		{"-0.125", eur, 2, LocaleENUS, "-0.13", nil},
		{"0.124", eur, 2, LocaleENUS, "0.12", nil},
		{"0.0000000049", usd, 8, LocaleENUS, "0.00000000", nil},
		{"0.0000000050", usd, 8, LocaleENUS, "0.00000001", nil},
		{"0.5", jpy, 0, LocaleENUS, "1", nil},
		{"abc", eur, 2, LocaleENUS, "", ErrInvalidAmount},
		{"", eur, 2, LocaleENUS, "", ErrInvalidAmount},
		{"1.2.3", eur, 2, LocaleENUS, "", ErrInvalidAmount},
		{"12345678901234567890123456789012345678901234567890123456789012345", eur, 2, LocaleENUS, "", ErrInvalidAmount},
		{"1", eur, 9, LocaleENUS, "", ErrInvalidValue},
		{"1", eur, 1, LocaleENUS, "", ErrInvalidValue},
		{"1", eur, 2, Locale("xx"), "", ErrInvalidValue},
		{"99999999999", usd, 8, LocaleENUS, "", ErrOverflow},
	}
	for _, tc := range tests {
		p, err := ParsePrice(tc.in, tc.c, tc.scale, tc.loc)
		if tc.err != nil {
			if !errors.Is(err, tc.err) {
				t.Errorf("ParsePrice(%q,%s,%d) err = %v, want %v", tc.in, tc.c, tc.scale, err, tc.err)
			}
			continue
		}
		if err != nil || p.String() != tc.want {
			t.Errorf("ParsePrice(%q,%s,%d) = %q, %v; want %q", tc.in, tc.c, tc.scale, p, err, tc.want)
		}
	}
}

func TestCheckOpAccount(t *testing.T) {
	accts := []AccountKind{
		AccountCash, AccountChecking, AccountCreditCard, AccountSavings, AccountInvestment, AccountLoan, AccountWallet,
	}
	secs := []SecurityKind{SecurityStock, SecurityETF, SecurityBond, SecurityFund, SecurityCrypto}
	for _, a := range accts {
		for _, s := range secs {
			want := a == AccountInvestment || a == AccountWallet && s == SecurityCrypto
			err := CheckOpAccount(a, s)
			if want && err != nil || !want && !errors.Is(err, ErrInvalidValue) {
				t.Errorf("CheckOpAccount(%s,%s) = %v, want ok=%v", a, s, err, want)
			}
		}
	}
	if k, err := ParseAccountKind("Wallet"); err != nil || k != AccountWallet {
		t.Errorf("ParseAccountKind(Wallet) = %v, %v", k, err)
	}
}

func TestNewSecurityPriceScale(t *testing.T) {
	base := Security{Name: "X", Ticker: "X"}
	tests := []struct {
		name  string
		kind  SecurityKind
		cur   string
		scale int
		want  int
		err   error
	}{
		{"stock default", SecurityStock, "EUR", 0, 2, nil},
		{"jpy stock default", SecurityStock, "JPY", 0, 0, nil},
		{"crypto default", SecurityCrypto, "USD", 0, 8, nil},
		{"explicit", SecurityETF, "EUR", 4, 4, nil},
		{"below decimals", SecurityETF, "EUR", 1, 0, ErrInvalidValue},
		{"above max", SecurityCrypto, "EUR", 9, 0, ErrInvalidValue},
	}
	for _, tc := range tests {
		s := base
		s.Kind, s.Currency, s.PriceScale = tc.kind, cur(t, tc.cur), tc.scale
		got, err := NewSecurity(s)
		if tc.err != nil {
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: err = %v, want %v", tc.name, err, tc.err)
			}
			continue
		}
		if err != nil || got.PriceScale != tc.want {
			t.Errorf("%s: PriceScale = %d, %v; want %d", tc.name, got.PriceScale, err, tc.want)
		}
	}
}

func TestQuantityValuePriceSubCent(t *testing.T) {
	// 2.5 BTC-like units at 0.00001234 USD = 0.00003085 → 0.00 USD; 1e8 units = 1234.00.
	small, err := qty(t, "2.5", 1).Value(priceOf(t, 1234, 8, "USD"))
	if err != nil || small.Minor() != 0 {
		t.Errorf("small = %v, %v", small, err)
	}
	big, err := qty(t, "100000000", 0).Value(priceOf(t, 1234, 8, "USD"))
	if err != nil || big.Minor() != 123400 {
		t.Errorf("big = %v, %v", big, err)
	}
}

func TestQuantityValuePriceProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		qs := rapid.IntRange(0, 8).Draw(t, "qscale")
		ps := rapid.IntRange(2, 8).Draw(t, "pscale")
		units := rapid.Int64Range(-1_000_000_000_000, 1_000_000_000_000).Draw(t, "units")
		minor := rapid.Int64Range(0, 1_000_000_000_000).Draw(t, "minor")
		q, err := NewQuantity(units, qs)
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewPrice(minor, ps, Currency{code: "EUR", decimals: 2})
		if err != nil {
			t.Fatal(err)
		}
		got, err := q.Value(p)

		exact := new(big.Rat).SetFrac(
			new(big.Int).Mul(big.NewInt(units), big.NewInt(minor)),
			new(big.Int).Mul(pow10(qs), pow10(ps-2)),
		)
		want, wantErr := RoundDiv(exact.Num(), exact.Denom())
		if (err != nil) != (wantErr != nil) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
		if err == nil && got.Minor() != want {
			t.Fatalf("%s × %s = %d, want %d", q, p, got.Minor(), want)
		}
	})
}
