package render

import (
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// FormatDate renders d for the table format, DD/MM/YYYY (it-IT) or MM/DD/YYYY (en-US).
func FormatDate(d domain.Date, loc domain.Locale) string {
	if d.IsZero() {
		return ""
	}
	return d.Format(loc)
}

// FormatDateMachine renders d for --csv: the locale-independent ISO form YYYY-MM-DD, per the
// technical spec's "Output" rule that --csv is independent of locale (the same convention --json
// already uses for dates).
func FormatDateMachine(d domain.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.String()
}

// FormatMoney renders m for the table format, per locale, with a leading "-" on outflows.
func FormatMoney(m domain.Money, loc domain.Locale) string {
	return domain.FormatAmount(m, loc)
}

// FormatMoneyMachine renders m for --csv: dot decimal, no thousands separator, leading "-" on
// outflows, independent of locale, per the technical spec's "Output" rule for --csv.
func FormatMoneyMachine(m domain.Money) string {
	return domain.FormatMachine(m)
}

// FormatPrice renders p for the table format with the locale's separators and every digit of its
// scale, so a sub-cent crypto price such as 0.00001234 is never truncated.
func FormatPrice(p domain.Price, loc domain.Locale) string {
	s := p.String()
	if loc != domain.LocaleITIT {
		return withGrouping(s, ".", ",")
	}
	return withGrouping(s, ",", ".")
}

// withGrouping swaps the "." decimal point of a machine number for dec and groups the integer
// digits by thousands with grp.
func withGrouping(s, dec, grp string) string {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(grp)
		}
		b.WriteRune(r)
	}
	out := sign + b.String()
	if hasFrac {
		out += dec + frac
	}
	return out
}

// FormatRate renders a scaled exchange rate with its significant decimals, e.g.
// 1084200000 -> "1.0842". It is the single place the CLI turns a domain.Rate into text.
func FormatRate(r domain.Rate) string {
	whole, frac := r.Scaled()/domain.RateScale, r.Scaled()%domain.RateScale
	if frac == 0 {
		return fmt.Sprint(whole)
	}
	return fmt.Sprintf("%d.%s", whole, strings.TrimRight(fmt.Sprintf("%09d", frac), "0"))
}
