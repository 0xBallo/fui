package reporting

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/0xballo/fui/internal/domain"
)

var (
	lastNMonths = regexp.MustCompile(`^last-([0-9]+)-months$`)
	absMonth    = regexp.MustCompile(`^([0-9]{4})-([0-9]{2})$`)
	absQuarter  = regexp.MustCompile(`^([0-9]{4})-[Qq]([0-9])$`)
	absYear     = regexp.MustCompile(`^[0-9]{4}$`)
)

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

// ValidPeriodSpec reports whether in is a period ResolvePeriodSpec understands: a relative
// period, YYYY-MM, YYYY-Qn (n 1..4) or YYYY.
func ValidPeriodSpec(in string) bool {
	if ValidRelativePeriod(in) {
		return true
	}
	_, err := resolveAbsolute(in, 1)
	return err == nil
}

// ResolvePeriodSpec resolves a relative period (see ResolveRelative) or an absolute one: YYYY-MM
// is the accounting month containing the first day of that month (so it honours startDay),
// YYYY-Qn is a calendar quarter and YYYY a calendar year.
func ResolvePeriodSpec(in string, today domain.Date, startDay int) (domain.Period, error) {
	if ValidRelativePeriod(in) {
		return ResolveRelative(in, today, startDay)
	}
	return resolveAbsolute(in, startDay)
}

func resolveAbsolute(in string, startDay int) (domain.Period, error) {
	bad := &domain.Error{
		Code: domain.CodeInvalidValue, Message: fmt.Sprintf("unknown period %q (expected a relative period, YYYY-MM, YYYY-Qn or YYYY)", in),
		Details: map[string]any{"field": "period", "value": in},
	}
	if absYear.MatchString(in) {
		y, _ := strconv.Atoi(in)
		p, err := yearPeriod(y)
		if err != nil {
			return domain.Period{}, bad
		}
		return p, nil
	}
	if m := absMonth.FindStringSubmatch(in); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		first, err := domain.NewDate(y, mo, 1)
		if err != nil {
			return domain.Period{}, bad
		}
		return domain.MonthPeriod(first, startDay)
	}
	if m := absQuarter.FindStringSubmatch(in); m != nil {
		y, _ := strconv.Atoi(m[1])
		q, _ := strconv.Atoi(m[2])
		if q < 1 || q > 4 {
			return domain.Period{}, bad
		}
		from, err := domain.NewDate(y, 3*q-2, 1)
		if err != nil {
			return domain.Period{}, bad
		}
		last, err := lastOfMonth(y, 3*q)
		if err != nil {
			return domain.Period{}, bad
		}
		return domain.NewPeriod(from, last)
	}
	return domain.Period{}, bad
}

// lastOfMonth is the last day of month m of year y.
func lastOfMonth(y, m int) (domain.Date, error) {
	for day := 31; day >= 28; day-- {
		if d, err := domain.NewDate(y, m, day); err == nil {
			return d, nil
		}
	}
	return domain.Date{}, fmt.Errorf("last day of %d-%02d: %w", y, m, domain.ErrInvalidValue)
}
