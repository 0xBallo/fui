package domain

import (
	"math/big"
)

// RoundDiv returns num/den rounded half away from zero. It is the only
// rounding in the domain: FX conversion and over-precise amount input both go
// through it, so the policy cannot drift. It fails with ErrOverflow if the
// result does not fit in an int64.
func RoundDiv(num, den *big.Int) (int64, error) {
	if den.Sign() == 0 {
		return 0, newError(ErrInvalidValue, nil, "division by zero")
	}
	q, r := new(big.Int).QuoRem(num, den, new(big.Int))
	// |2r| >= |den| means the discarded fraction is at least one half.
	if r.Abs(r).Lsh(r, 1).CmpAbs(den) >= 0 {
		if num.Sign()*den.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	if !q.IsInt64() {
		return 0, newError(ErrOverflow, nil, "result out of range")
	}
	return q.Int64(), nil
}

// pow10 returns 10^n for n >= 0.
func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
