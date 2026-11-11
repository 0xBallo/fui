package domain

import (
	"bytes"
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"

	"pgregory.net/rapid" //nolint:depguard // test-only dependency allowed by the technical spec
)

func TestNewMoneyRequiresCurrency(t *testing.T) {
	if _, err := NewMoney(1, Currency{}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("err = %v", err)
	}
}

func TestMoneyArithmetic(t *testing.T) {
	eur := func(n int64) Money { return money(t, n, "EUR") }
	usd := money(t, 1, "USD")
	hi, lo := eur(math.MaxInt64), eur(math.MinInt64)

	tests := []struct {
		name    string
		got     func() (Money, error)
		want    Money
		wantErr error
	}{
		{"add", func() (Money, error) { return eur(150).Add(eur(-250)) }, eur(-100), nil},
		{"sub", func() (Money, error) { return eur(150).Sub(eur(250)) }, eur(-100), nil},
		{"neg", func() (Money, error) { return eur(150).Neg() }, eur(-150), nil},
		{"neg zero", func() (Money, error) { return eur(0).Neg() }, eur(0), nil},
		{"add mismatch", func() (Money, error) { return eur(1).Add(usd) }, Money{}, ErrCurrencyMismatch},
		{"sub mismatch", func() (Money, error) { return eur(1).Sub(usd) }, Money{}, ErrCurrencyMismatch},
		{"add overflow", func() (Money, error) { return hi.Add(eur(1)) }, Money{}, ErrOverflow},
		{"add underflow", func() (Money, error) { return lo.Add(eur(-1)) }, Money{}, ErrOverflow},
		{"add extremes ok", func() (Money, error) { return hi.Add(lo) }, eur(-1), nil},
		{"sub overflow", func() (Money, error) { return hi.Sub(eur(-1)) }, Money{}, ErrOverflow},
		{"sub underflow", func() (Money, error) { return lo.Sub(eur(1)) }, Money{}, ErrOverflow},
		{"neg min", func() (Money, error) { return lo.Neg() }, Money{}, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestMoneyPredicates(t *testing.T) {
	a, b := money(t, -5, "EUR"), money(t, 7, "EUR")
	if a.Sign() != -1 || b.Sign() != 1 || money(t, 0, "EUR").Sign() != 0 {
		t.Error("Sign")
	}
	if !money(t, 0, "EUR").IsZero() || a.IsZero() {
		t.Error("IsZero")
	}
	for _, tt := range []struct {
		x, y Money
		want int
	}{{a, b, -1}, {b, a, 1}, {a, a, 0}} {
		if got, err := tt.x.Cmp(tt.y); err != nil || got != tt.want {
			t.Errorf("Cmp(%v,%v) = %d, %v", tt.x, tt.y, got, err)
		}
	}
	if _, err := a.Cmp(money(t, 1, "USD")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Cmp mismatch err = %v", err)
	}
	if a.Minor() != -5 || a.Currency().Code() != "EUR" {
		t.Error("accessors")
	}
}

func TestMoneyStringAndRedaction(t *testing.T) {
	m := money(t, -123456, "EUR")
	if got := m.String(); got != "-1234.56 EUR" {
		t.Errorf("String = %q", got)
	}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("x", "amount", m)
	if out := buf.String(); !strings.Contains(out, "[redacted]") || strings.Contains(out, "1234") {
		t.Errorf("log output leaks amount: %s", out)
	}
}

func TestMoneyAddSubProperties(t *testing.T) {
	eur := cur(t, "EUR")
	rapid.Check(t, func(t *rapid.T) {
		// Bounded so no intermediate sum can overflow; overflow is tested explicitly above.
		gen := rapid.Int64Range(-1<<40, 1<<40)
		mk := func(n int64) Money { m, _ := NewMoney(n, eur); return m }
		a, b, c := mk(gen.Draw(t, "a")), mk(gen.Draw(t, "b")), mk(gen.Draw(t, "c"))
		ab, _ := a.Add(b)
		left, _ := ab.Add(c)
		bc, _ := b.Add(c)
		right, _ := a.Add(bc)
		if left != right {
			t.Fatalf("(a+b)+c = %v, a+(b+c) = %v", left, right)
		}
		back, _ := ab.Sub(b)
		if back != a {
			t.Fatalf("(a+b)-b = %v, want %v", back, a)
		}
	})
}
