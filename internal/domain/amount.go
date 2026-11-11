package domain

import (
	"math/big"
	"strconv"
	"strings"
)

// maxAmountLen bounds input size so hostile input cannot force huge big-number work.
const maxAmountLen = 64

// ParseAmount parses user input into Money of currency c. It is the single
// amount parser for CLI and TUI. Accepted: optional sign, digits with
// optional thousands separators (groups of exactly three), and an optional
// decimal part, using the separators of loc. Input with more decimals than c
// is rounded half away from zero. Anything else, including the other
// locale's separators in the wrong place, fails with ErrInvalidAmount.
func ParseAmount(s string, loc Locale, c Currency) (Money, error) {
	if !loc.Valid() {
		return Money{}, invalidValue("locale", string(loc))
	}
	if c.IsZero() {
		return Money{}, missingField("currency")
	}
	in := strings.TrimSpace(s)
	if len(in) > maxAmountLen {
		return Money{}, invalidAmount(s)
	}
	neg := false
	if in != "" && (in[0] == '+' || in[0] == '-') {
		neg = in[0] == '-'
		in = in[1:]
	}
	digits, frac, ok := splitAmount(in, loc)
	if !ok {
		return Money{}, invalidAmount(s)
	}
	num, _ := new(big.Int).SetString(digits+frac, 10) // digits validated by splitAmount
	if neg {
		num.Neg(num)
	}
	// Scale to c's decimals; any excess fraction digits are rounded away.
	den := big.NewInt(1)
	if excess := len(frac) - c.Decimals(); excess > 0 {
		den = pow10(excess)
	} else {
		num.Mul(num, pow10(-excess))
	}
	minor, err := RoundDiv(num, den)
	if err != nil {
		return Money{}, err
	}
	return NewMoney(minor, c)
}

// splitAmount validates the unsigned body and returns the integer digits
// (thousands separators removed) and the fraction digits.
func splitAmount(in string, loc Locale) (digits, frac string, ok bool) {
	dec, grp := loc.separators()
	intPart, frac, hasDec := strings.Cut(in, dec)
	if hasDec && (frac == "" || !allDigits(frac)) {
		return "", "", false
	}
	groups := strings.Split(intPart, grp)
	for i, g := range groups {
		if !allDigits(g) {
			return "", "", false
		}
		if len(groups) > 1 && (i == 0 && len(g) > 3 || i > 0 && len(g) != 3) {
			return "", "", false
		}
	}
	return strings.Join(groups, ""), frac, true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func invalidAmount(input string) error {
	return newError(ErrInvalidAmount, map[string]any{"input": input}, "invalid amount %q", input)
}

// FormatAmount renders m for display in loc: thousands separators, the
// locale's decimal separator, leading "-" for negatives, no currency symbol.
// ParseAmount(FormatAmount(m, loc), loc, m.Currency()) returns m.
func FormatAmount(m Money, loc Locale) string {
	dec, grp := loc.separators()
	return formatMinor(m.minor, m.cur.Decimals(), dec, grp)
}

// FormatMachine renders m for scripts: "-1234.56", dot decimal, no grouping.
// It parses back with ParseAmount in LocaleENUS.
func FormatMachine(m Money) string {
	return formatMinor(m.minor, m.cur.Decimals(), ".", "")
}

func formatMinor(minor int64, decimals int, dec, grp string) string {
	u := uint64(minor) //nolint:gosec // reinterpreting bits; negated below for negatives
	if minor < 0 {
		u = -u // two's complement negate also handles math.MinInt64
	}
	digits := strconv.FormatUint(u, 10)
	if pad := decimals + 1 - len(digits); pad > 0 {
		digits = strings.Repeat("0", pad) + digits
	}
	intPart, frac := digits[:len(digits)-decimals], digits[len(digits)-decimals:]
	if grp != "" {
		intPart = groupThousands(intPart, grp)
	}
	var b strings.Builder
	if minor < 0 {
		b.WriteByte('-')
	}
	b.WriteString(intPart)
	if decimals > 0 {
		b.WriteString(dec)
		b.WriteString(frac)
	}
	return b.String()
}

func groupThousands(digits, sep string) string {
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(r)
	}
	return b.String()
}
