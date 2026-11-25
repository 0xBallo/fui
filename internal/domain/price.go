package domain

import (
	"math/big"
	"strings"
)

// Price is an exact unit price: an int64 count of 10^-scale units of a
// currency. Unlike Money it may carry more decimals than the currency (up to
// MaxQuantityScale), which crypto prices need (e.g. 0.00001234 USD).
type Price struct {
	minor int64
	scale int
	cur   Currency
}

// NewPrice returns minor × 10^-scale of c, failing if c is the zero Currency
// or scale is outside c.Decimals()..MaxQuantityScale.
func NewPrice(minor int64, scale int, c Currency) (Price, error) {
	if c.IsZero() {
		return Price{}, missingField("currency")
	}
	if scale < c.Decimals() || scale > MaxQuantityScale {
		return Price{}, newError(ErrInvalidValue, map[string]any{"field": "price scale", "value": scale},
			"price scale %d out of range %d..%d for %s", scale, c.Decimals(), MaxQuantityScale, c)
	}
	return Price{minor: minor, scale: scale, cur: c}, nil
}

// Minor returns the price as an integer count of 10^-Scale currency units.
func (p Price) Minor() int64 { return p.minor }

// Scale returns the number of decimals.
func (p Price) Scale() int { return p.scale }

// Currency returns the currency of p.
func (p Price) Currency() Currency { return p.cur }

// Sign returns -1, 0 or +1.
func (p Price) Sign() int {
	switch {
	case p.minor < 0:
		return -1
	case p.minor > 0:
		return 1
	}
	return 0
}

// IsZero reports whether the price amount is zero.
func (p Price) IsZero() bool { return p.minor == 0 }

// minorRat returns the exact value of p in minor units of its currency.
func (p Price) minorRat() *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(p.minor), pow10(p.scale-p.cur.Decimals()))
}

// String formats p with exactly Scale decimals, without currency symbol.
func (p Price) String() string {
	return formatMinor(p.minor, p.scale, ".", "")
}

// ParsePrice parses user input into a Price of currency c with scale decimals.
// The grammar is that of ParseAmount; digits beyond scale are rounded half
// away from zero.
func ParsePrice(s string, c Currency, scale int, loc Locale) (Price, error) {
	if !loc.Valid() {
		return Price{}, invalidValue("locale", string(loc))
	}
	if _, err := NewPrice(0, scale, c); err != nil {
		return Price{}, err
	}
	in := strings.TrimSpace(s)
	if len(in) > maxAmountLen {
		return Price{}, invalidAmount(s)
	}
	neg := false
	if in != "" && (in[0] == '+' || in[0] == '-') {
		neg = in[0] == '-'
		in = in[1:]
	}
	digits, frac, ok := splitAmount(in, loc)
	if !ok {
		return Price{}, invalidAmount(s)
	}
	num, _ := new(big.Int).SetString(digits+frac, 10) // digits validated by splitAmount
	if neg {
		num.Neg(num)
	}
	den := big.NewInt(1)
	if excess := len(frac) - scale; excess > 0 {
		den = pow10(excess)
	} else {
		num.Mul(num, pow10(-excess))
	}
	minor, err := RoundDiv(num, den)
	if err != nil {
		return Price{}, err
	}
	return NewPrice(minor, scale, c)
}
