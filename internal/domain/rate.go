package domain

import (
	"math/big"
)

// RateScale is the fixed-point scale of Rate: 1.0 is stored as 1_000_000_000.
const RateScale int64 = 1_000_000_000

// Rate is an exchange rate scaled by RateScale: units of the target currency
// per one unit of the source currency. A Rate from NewRate is always positive.
type Rate int64

// NewRate returns the Rate with the given scaled value, which must be positive.
func NewRate(scaled int64) (Rate, error) {
	if scaled <= 0 {
		return 0, newError(ErrInvalidValue, map[string]any{"field": "rate", "value": scaled},
			"rate must be positive, got %d", scaled)
	}
	return Rate(scaled), nil
}

// Scaled returns the raw scaled value.
func (r Rate) Scaled() int64 { return int64(r) }

// Convert converts m to currency to at rate r, rounding exactly once (half
// away from zero) at the minor-unit precision of the target currency, so the
// decimals of both currencies are respected (e.g. EUR → JPY).
func Convert(m Money, r Rate, to Currency) (Money, error) {
	if r <= 0 {
		return Money{}, newError(ErrInvalidValue, map[string]any{"field": "rate", "value": int64(r)},
			"rate must be positive, got %d", int64(r))
	}
	if m.cur.IsZero() || to.IsZero() {
		return Money{}, missingField("currency")
	}
	// result = minor · rate · 10^toDecimals / (10^fromDecimals · RateScale), one division only.
	num := new(big.Int).Mul(big.NewInt(m.minor), big.NewInt(int64(r)))
	num.Mul(num, pow10(to.Decimals()))
	den := pow10(m.cur.Decimals())
	den.Mul(den, big.NewInt(RateScale))
	minor, err := RoundDiv(num, den)
	if err != nil {
		return Money{}, err
	}
	return NewMoney(minor, to)
}
