package domain

import (
	"math/big"
	"strings"
)

// MaxQuantityScale is the largest number of decimals a Quantity may carry.
const MaxQuantityScale = 8

// Quantity is an exact amount of a security: an int64 count of 10^-scale
// units (scale 0 for shares, up to 8 for crypto). Operations are only defined
// between quantities with the same scale.
type Quantity struct {
	units int64
	scale int
}

// NewQuantity returns units × 10^-scale, failing if scale is outside 0..MaxQuantityScale.
func NewQuantity(units int64, scale int) (Quantity, error) {
	if scale < 0 || scale > MaxQuantityScale {
		return Quantity{}, newError(ErrInvalidValue, map[string]any{"field": "quantity scale", "value": scale},
			"quantity scale %d out of range 0..%d", scale, MaxQuantityScale)
	}
	return Quantity{units: units, scale: scale}, nil
}

// Units returns the quantity as an integer count of 10^-Scale units.
func (q Quantity) Units() int64 { return q.units }

// Scale returns the number of decimals.
func (q Quantity) Scale() int { return q.scale }

// Sign returns -1, 0 or +1.
func (q Quantity) Sign() int {
	switch {
	case q.units < 0:
		return -1
	case q.units > 0:
		return 1
	}
	return 0
}

// IsZero reports whether the quantity is zero.
func (q Quantity) IsZero() bool { return q.units == 0 }

func (q Quantity) sameScale(o Quantity) error {
	if q.scale != o.scale {
		return newError(ErrInvalidValue, map[string]any{"left": q.scale, "right": o.scale},
			"quantity scale mismatch: %d vs %d", q.scale, o.scale)
	}
	return nil
}

// Add returns q+o, failing on scale mismatch or overflow.
func (q Quantity) Add(o Quantity) (Quantity, error) {
	if err := q.sameScale(o); err != nil {
		return Quantity{}, err
	}
	s := q.units + o.units
	if (q.units^s)&(o.units^s) < 0 {
		return Quantity{}, newError(ErrOverflow, nil, "quantity addition overflows")
	}
	return Quantity{units: s, scale: q.scale}, nil
}

// Sub returns q-o, failing on scale mismatch or overflow.
func (q Quantity) Sub(o Quantity) (Quantity, error) {
	if err := q.sameScale(o); err != nil {
		return Quantity{}, err
	}
	d := q.units - o.units
	if (q.units^o.units)&(q.units^d) < 0 {
		return Quantity{}, newError(ErrOverflow, nil, "quantity subtraction overflows")
	}
	return Quantity{units: d, scale: q.scale}, nil
}

// Neg returns -q; it overflows only for the minimum int64.
func (q Quantity) Neg() (Quantity, error) {
	if q.units == -q.units && q.units != 0 {
		return Quantity{}, newError(ErrOverflow, nil, "quantity negation overflows")
	}
	return Quantity{units: -q.units, scale: q.scale}, nil
}

// Cmp compares quantities of the same scale, returning -1, 0 or +1.
func (q Quantity) Cmp(o Quantity) (int, error) {
	if err := q.sameScale(o); err != nil {
		return 0, err
	}
	switch {
	case q.units < o.units:
		return -1, nil
	case q.units > o.units:
		return 1, nil
	}
	return 0, nil
}

// rat returns the exact value of q.
func (q Quantity) rat() *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(q.units), pow10(q.scale))
}

// Value returns q × price as Money in the price currency, with a single
// rounding (half away from zero) of the exact product.
func (q Quantity) Value(price Money) (Money, error) {
	v, err := roundRat(new(big.Rat).Mul(q.rat(), new(big.Rat).SetInt64(price.minor)))
	if err != nil {
		return Money{}, err
	}
	return NewMoney(v, price.cur)
}

// roundRat rounds r half away from zero to an int64.
func roundRat(r *big.Rat) (int64, error) {
	return RoundDiv(r.Num(), r.Denom())
}

// String formats q as a plain decimal with exactly Scale decimals.
func (q Quantity) String() string {
	neg := q.units < 0
	digits := new(big.Int).Abs(big.NewInt(q.units)).String()
	if q.scale > 0 {
		if len(digits) <= q.scale {
			digits = strings.Repeat("0", q.scale-len(digits)+1) + digits
		}
		cut := len(digits) - q.scale
		digits = digits[:cut] + "." + digits[cut:]
	}
	if neg {
		return "-" + digits
	}
	return digits
}

// ParseQuantity parses a plain decimal ("12", "-0.5", "1.00000001", "." as
// separator) into a Quantity of the given scale. Input with more decimals
// than scale is rejected rather than rounded.
func ParseQuantity(s string, scale int) (Quantity, error) {
	if _, err := NewQuantity(0, scale); err != nil {
		return Quantity{}, err
	}
	orig := s
	s = strings.TrimSpace(s)
	neg := false
	switch {
	case strings.HasPrefix(s, "-"):
		neg, s = true, s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	intPart, frac, _ := strings.Cut(s, ".")
	if intPart == "" && frac == "" || !digitsOrEmpty(intPart) || !digitsOrEmpty(frac) {
		return Quantity{}, newError(ErrInvalidAmount, map[string]any{"value": orig}, "invalid quantity %q", orig)
	}
	if len(frac) > scale {
		return Quantity{}, newError(ErrInvalidAmount, map[string]any{"value": orig, "scale": scale},
			"quantity %q has more than %d decimals", orig, scale)
	}
	frac += strings.Repeat("0", scale-len(frac))
	n, ok := new(big.Int).SetString(intPart+frac, 10)
	if !ok {
		return Quantity{}, newError(ErrInvalidAmount, map[string]any{"value": orig}, "invalid quantity %q", orig)
	}
	if neg {
		n.Neg(n)
	}
	if !n.IsInt64() {
		return Quantity{}, newError(ErrOverflow, map[string]any{"value": orig}, "quantity %q out of range", orig)
	}
	return Quantity{units: n.Int64(), scale: scale}, nil
}

func digitsOrEmpty(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
