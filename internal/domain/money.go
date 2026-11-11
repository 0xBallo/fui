package domain

import (
	"log/slog"
)

// Money is an exact amount: int64 minor units (cents) plus a Currency. There
// is no implicit conversion; mixing currencies is an error. A Money built with
// NewMoney is always valid; the zero Money has no currency.
type Money struct {
	minor int64
	cur   Currency
}

// NewMoney returns minor units of c. It fails if c is the zero Currency.
func NewMoney(minor int64, c Currency) (Money, error) {
	if c.IsZero() {
		return Money{}, missingField("currency")
	}
	return Money{minor: minor, cur: c}, nil
}

// Minor returns the amount in minor units (e.g. cents).
func (m Money) Minor() int64 { return m.minor }

// Currency returns the currency of m.
func (m Money) Currency() Currency { return m.cur }

// Add returns m+o, failing on currency mismatch or int64 overflow.
func (m Money) Add(o Money) (Money, error) {
	if err := m.sameCurrency(o); err != nil {
		return Money{}, err
	}
	s := m.minor + o.minor
	if (m.minor^s)&(o.minor^s) < 0 {
		return Money{}, newError(ErrOverflow, nil, "%s + %s overflows", m.cur, o.cur)
	}
	return Money{minor: s, cur: m.cur}, nil
}

// Sub returns m-o, failing on currency mismatch or int64 overflow.
func (m Money) Sub(o Money) (Money, error) {
	if err := m.sameCurrency(o); err != nil {
		return Money{}, err
	}
	d := m.minor - o.minor
	if (m.minor^o.minor)&(m.minor^d) < 0 {
		return Money{}, newError(ErrOverflow, nil, "%s - %s overflows", m.cur, o.cur)
	}
	return Money{minor: d, cur: m.cur}, nil
}

// Neg returns -m; it overflows only for the minimum int64.
func (m Money) Neg() (Money, error) {
	if m.minor == -m.minor && m.minor != 0 {
		return Money{}, newError(ErrOverflow, nil, "negation overflows")
	}
	return Money{minor: -m.minor, cur: m.cur}, nil
}

// Sign returns -1, 0 or +1.
func (m Money) Sign() int {
	switch {
	case m.minor < 0:
		return -1
	case m.minor > 0:
		return 1
	}
	return 0
}

// IsZero reports whether the amount is zero (in any currency).
func (m Money) IsZero() bool { return m.minor == 0 }

// Cmp compares amounts of the same currency, returning -1, 0 or +1.
func (m Money) Cmp(o Money) (int, error) {
	if err := m.sameCurrency(o); err != nil {
		return 0, err
	}
	switch {
	case m.minor < o.minor:
		return -1, nil
	case m.minor > o.minor:
		return 1, nil
	}
	return 0, nil
}

func (m Money) sameCurrency(o Money) error {
	if m.cur != o.cur {
		return newError(ErrCurrencyMismatch, map[string]any{"left": m.cur.Code(), "right": o.cur.Code()},
			"currency mismatch: %s vs %s", m.cur, o.cur)
	}
	return nil
}

// String returns the clear value in machine format plus currency, e.g.
// "-12.34 EUR". Unlike LogValue it is never redacted; the logging layer
// chooses between the two (FUI_LOG_SENSITIVE).
func (m Money) String() string { return FormatMachine(m) + " " + m.cur.Code() }

// LogValue implements slog.LogValuer: amounts are sensitive and logged as "[redacted]".
func (Money) LogValue() slog.Value { return slog.StringValue("[redacted]") }
