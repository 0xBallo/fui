package reporting

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/0xballo/fui/internal/domain"
)

var lastNMonths = regexp.MustCompile(`^last-([0-9]+)-months$`)

// ValidRelativePeriod reports whether in is a period ResolveRelative understands.
func ValidRelativePeriod(in string) bool {
	switch in {
	case "this-month", "last-month", "this-year", "last-year":
		return true
	}
	m := lastNMonths.FindStringSubmatch(in)
	if m == nil {
		return false
	}
	n, err := strconv.Atoi(m[1])
	return err == nil && n >= 1 && n <= 600
}

// ResolveRelative turns a relative period into dates as of today: this-month and last-month
// follow the accounting month (starting on startDay), this-year and last-year are calendar
// years, and last-N-months is the N complete accounting months before the current one, so that
// last-1-months equals last-month.
func ResolveRelative(in string, today domain.Date, startDay int) (domain.Period, error) {
	cur, err := domain.MonthPeriod(today, startDay)
	if err != nil {
		return domain.Period{}, err
	}
	switch in {
	case "this-month":
		return cur, nil
	case "last-month":
		return ResolveRelative("last-1-months", today, startDay)
	case "this-year", "last-year":
		y := today.Year()
		if in == "last-year" {
			y--
		}
		return yearPeriod(y)
	}
	m := lastNMonths.FindStringSubmatch(in)
	if m == nil || !ValidRelativePeriod(in) {
		return domain.Period{}, &domain.Error{
			Code: domain.CodeInvalidValue, Message: fmt.Sprintf("unknown relative period %q", in),
			Details: map[string]any{"field": "period", "value": in},
		}
	}
	n, _ := strconv.Atoi(m[1])
	to, from := cur.From(), cur.From()
	for i := range n {
		prev, err := domain.MonthPeriod(mustPrev(from), startDay)
		if err != nil {
			return domain.Period{}, err
		}
		if i == 0 {
			to = prev.To()
		}
		from = prev.From()
	}
	return domain.NewPeriod(from, to)
}

// mustPrev is the day before d; d is a month start inside the supported range.
func mustPrev(d domain.Date) domain.Date {
	p, _ := d.AddDays(-1)
	return p
}

func yearPeriod(year int) (domain.Period, error) {
	from, err := domain.NewDate(year, 1, 1)
	if err != nil {
		return domain.Period{}, err
	}
	to, err := domain.NewDate(year, 12, 31)
	if err != nil {
		return domain.Period{}, err
	}
	return domain.NewPeriod(from, to)
}
