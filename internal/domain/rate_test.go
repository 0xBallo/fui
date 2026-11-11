package domain

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"pgregory.net/rapid"
)

func rate(t tb, scaled int64) Rate {
	t.Helper()
	r, err := NewRate(scaled)
	if err != nil {
		t.Fatalf("NewRate(%d): %v", scaled, err)
	}
	return r
}

func TestNewRate(t *testing.T) {
	if r, err := NewRate(RateScale); err != nil || r.Scaled() != RateScale {
		t.Errorf("NewRate = %v, %v", r, err)
	}
	for _, bad := range []int64{0, -1, math.MinInt64} {
		if _, err := NewRate(bad); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("NewRate(%d) err = %v", bad, err)
		}
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name string
		m    Money
		r    int64
		to   string
		want int64
	}{
		{"eur to usd", money(t, 10000, "EUR"), 1_085_000_000, "USD", 10850},
		{"identity", money(t, 12345, "EUR"), RateScale, "EUR", 12345},
		{"rounds half up", money(t, 1, "EUR"), 500_000_000, "USD", 1},         // 0.5 → 1
		{"rounds half away neg", money(t, -1, "EUR"), 500_000_000, "USD", -1}, // -0.5 → -1
		{"rounds down below half", money(t, 1, "EUR"), 499_999_999, "USD", 0},
		{"eur to jpy drops decimals", money(t, 10000, "EUR"), 160_000_000_000, "JPY", 16000},
		{"jpy to eur adds decimals", money(t, 16000, "JPY"), 6_250_000, "EUR", 10000},
		{"single rounding", money(t, 333, "EUR"), 333_333_333, "USD", 111}, // 3.33 * 0.333333333 = 1.1099999… → 1.11
		{"zero", money(t, 0, "EUR"), RateScale, "USD", 0},
		{"large amount no overflow in middle", money(t, math.MaxInt64/2, "EUR"), 2 * RateScale, "USD", math.MaxInt64 - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.m, rate(t, tt.r), cur(t, tt.to))
			if err != nil || got.Minor() != tt.want || got.Currency().Code() != tt.to {
				t.Fatalf("got %v, %v; want %d %s", got, err, tt.want, tt.to)
			}
		})
	}
}

func TestConvertErrors(t *testing.T) {
	eur, usd := money(t, math.MaxInt64, "EUR"), cur(t, "USD")
	if _, err := Convert(eur, rate(t, 2*RateScale), usd); !errors.Is(err, ErrOverflow) {
		t.Errorf("overflow err = %v", err)
	}
	if _, err := Convert(eur, 0, usd); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("zero rate err = %v", err)
	}
	if _, err := Convert(eur, rate(t, RateScale), Currency{}); !errors.Is(err, ErrMissingField) {
		t.Errorf("zero target err = %v", err)
	}
	if _, err := Convert(Money{}, rate(t, RateScale), usd); !errors.Is(err, ErrMissingField) {
		t.Errorf("zero source err = %v", err)
	}
}

func TestConvertRoundsHalfAwayFromZeroProperty(t *testing.T) {
	eur, usd := cur(t, "EUR"), cur(t, "USD")
	rapid.Check(t, func(t *rapid.T) {
		minor := rapid.Int64Range(-1<<40, 1<<40).Draw(t, "minor")
		scaled := rapid.Int64Range(1, 100*RateScale).Draw(t, "rate")
		m, _ := NewMoney(minor, eur)
		got, err := Convert(m, Rate(scaled), usd)
		if err != nil {
			t.Fatal(err)
		}
		// Exact rational value; the result must be its nearest integer, ties away from zero.
		exact := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(minor), big.NewInt(scaled)), big.NewInt(RateScale))
		diff := new(big.Rat).Sub(exact, new(big.Rat).SetInt64(got.Minor()))
		half := big.NewRat(1, 2)
		if diff.Abs(diff).Cmp(half) > 0 {
			t.Fatalf("convert(%d, %d) = %d is more than half a unit from %v", minor, scaled, got.Minor(), exact)
		}
		// On an exact tie the result is the farther of the two neighbours from zero.
		if diff.Cmp(half) == 0 {
			absGot, absExact := new(big.Rat).SetInt64(got.Minor()), new(big.Rat).Set(exact)
			if absGot.Abs(absGot).Cmp(absExact.Abs(absExact)) <= 0 {
				t.Fatalf("tie for %v rounded toward zero to %d", exact, got.Minor())
			}
		}
		// Odd sign symmetry: converting the negation negates the result.
		neg, _ := NewMoney(-minor, eur)
		gotNeg, _ := Convert(neg, Rate(scaled), usd)
		if gotNeg.Minor() != -got.Minor() {
			t.Fatalf("asymmetric: f(%d)=%d, f(%d)=%d", minor, got.Minor(), -minor, gotNeg.Minor())
		}
	})
}
