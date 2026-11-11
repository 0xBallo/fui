package domain

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"pgregory.net/rapid" //nolint:depguard // test-only dependency allowed by the technical spec
)

func TestRoundDiv(t *testing.T) {
	tests := []struct {
		num, den int64
		want     int64
	}{
		{5, 2, 3},
		{-5, 2, -3},
		{5, -2, -3},
		{-5, -2, 3},
		{4, 2, 2},
		{7, 3, 2},
		{-7, 3, -2},
		{1, 3, 0},
		{-1, 3, 0},
		{149, 100, 1},
		{150, 100, 2},
		{-149, 100, -1},
		{-150, 100, -2},
		{0, 7, 0},
	}
	for _, tt := range tests {
		got, err := RoundDiv(big.NewInt(tt.num), big.NewInt(tt.den))
		if err != nil || got != tt.want {
			t.Errorf("RoundDiv(%d,%d) = %d, %v; want %d", tt.num, tt.den, got, err, tt.want)
		}
	}
}

func TestRoundDivErrors(t *testing.T) {
	if _, err := RoundDiv(big.NewInt(1), big.NewInt(0)); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("zero den err = %v", err)
	}
	huge := new(big.Int).Lsh(big.NewInt(1), 70)
	if _, err := RoundDiv(huge, big.NewInt(1)); !errors.Is(err, ErrOverflow) {
		t.Errorf("overflow err = %v", err)
	}
	if got, err := RoundDiv(big.NewInt(math.MinInt64), big.NewInt(1)); err != nil || got != math.MinInt64 {
		t.Errorf("min int64 = %d, %v", got, err)
	}
}

func TestRoundDivHalfAwayFromZeroProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64Range(-1<<40, 1<<40).Draw(t, "n")
		d := rapid.Int64Range(1, 1<<20).Draw(t, "d")
		got, err := RoundDiv(big.NewInt(n), big.NewInt(d))
		if err != nil {
			t.Fatal(err)
		}
		// Reference: floor((2|n|+d) / 2d) with the sign of n, in pure integers.
		abs := n
		if abs < 0 {
			abs = -abs
		}
		want := (2*abs + d) / (2 * d)
		if n < 0 {
			want = -want
		}
		if got != want {
			t.Fatalf("RoundDiv(%d,%d) = %d, want %d", n, d, got, want)
		}
		neg, _ := RoundDiv(big.NewInt(-n), big.NewInt(d))
		if neg != -got {
			t.Fatalf("not symmetric: f(%d)=%d, f(%d)=%d", n, got, -n, neg)
		}
	})
}
