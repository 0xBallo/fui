package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	minYear = 1
	maxYear = 9999
	// maxShift bounds AddDays/AddMonths/relative offsets so arithmetic cannot overflow.
	maxShiftDays   = 4_000_000
	maxShiftMonths = 120_000
)

// Date is a calendar day without time of day or time zone. Time zones only
// matter at the edges (FromTime, In). A Date built by a constructor is always
// valid; the zero Date is "unset".
type Date struct {
	year  int
	month int
	day   int
}

// NewDate returns the date y-m-d, failing with ErrInvalidDate if it does not exist.
func NewDate(y, m, d int) (Date, error) {
	if y < minYear || y > maxYear || m < 1 || m > 12 || d < 1 || d > daysIn(y, m) {
		return Date{}, newError(ErrInvalidDate, nil, "invalid date %04d-%02d-%02d", y, m, d)
	}
	return Date{year: y, month: m, day: d}, nil
}

func daysIn(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// FromTime returns the calendar day of t in t's own location.
func FromTime(t time.Time) (Date, error) {
	y, m, d := t.Date()
	return NewDate(y, int(m), d)
}

// In returns midnight of d in loc.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.year, time.Month(d.month), d.day, 0, 0, 0, 0, loc)
}

// Year returns the year.
func (d Date) Year() int { return d.year }

// Month returns the month, 1..12.
func (d Date) Month() int { return d.month }

// Day returns the day of the month.
func (d Date) Day() int { return d.day }

// IsZero reports whether d is the unset zero Date.
func (d Date) IsZero() bool { return d == Date{} }

// Cmp returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Cmp(o Date) int {
	switch {
	case d.year != o.year:
		return sign(d.year - o.year)
	case d.month != o.month:
		return sign(d.month - o.month)
	}
	return sign(d.day - o.day)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// Before reports whether d is strictly before o.
func (d Date) Before(o Date) bool { return d.Cmp(o) < 0 }

// After reports whether d is strictly after o.
func (d Date) After(o Date) bool { return d.Cmp(o) > 0 }

// AddDays returns d shifted by n days (negative goes back).
func (d Date) AddDays(n int) (Date, error) {
	if n > maxShiftDays || n < -maxShiftDays {
		return Date{}, newError(ErrInvalidDate, nil, "cannot shift a date by %d days", n)
	}
	// UTC avoids any DST/zone effect; time.Time is not retained.
	y, m, day := time.Date(d.year, time.Month(d.month), d.day+n, 0, 0, 0, 0, time.UTC).Date()
	return NewDate(y, int(m), day)
}

// AddMonths returns d shifted by n months; a day past the end of the target
// month is clamped to its last day (31 Jan + 1 month = 28/29 Feb).
func (d Date) AddMonths(n int) (Date, error) {
	if n > maxShiftMonths || n < -maxShiftMonths {
		return Date{}, newError(ErrInvalidDate, nil, "cannot shift a date by %d months", n)
	}
	total := d.year*12 + (d.month - 1) + n
	if total < 0 {
		return Date{}, newError(ErrInvalidDate, nil, "date out of range")
	}
	y, m := total/12, total%12+1
	return NewDate(y, m, min(d.day, daysIn(y, m)))
}

// String returns the ISO form YYYY-MM-DD.
func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day) }

// Format renders d for display: DD/MM/YYYY (it-IT) or MM/DD/YYYY (en-US).
func (d Date) Format(loc Locale) string {
	if loc == LocaleITIT {
		return fmt.Sprintf("%02d/%02d/%04d", d.day, d.month, d.year)
	}
	return fmt.Sprintf("%02d/%02d/%04d", d.month, d.day, d.year)
}

// ParseDate parses a strict ISO date, YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	return parseParts(s, "-", [3]int{0, 1, 2}, [3]int{4, 2, 2})
}

// ParseLocalDate parses the display form produced by Date.Format.
func ParseLocalDate(s string, loc Locale) (Date, error) {
	if loc == LocaleITIT {
		return parseParts(s, "/", [3]int{2, 1, 0}, [3]int{4, 2, 2})
	}
	return parseParts(s, "/", [3]int{2, 0, 1}, [3]int{4, 2, 2})
}

// parseParts splits s on sep into exactly three all-digit parts whose lengths
// are widths (in year, month, day order) and builds the date. order[i] is the
// position of the year (i=0), month (1) and day (2) within s.
func parseParts(s, sep string, order, widths [3]int) (Date, error) {
	parts := strings.Split(s, sep)
	if len(parts) != 3 {
		return Date{}, invalidDate(s)
	}
	var v [3]int
	for i := range v {
		p := parts[order[i]]
		if len(p) != widths[i] || !allDigits(p) {
			return Date{}, invalidDate(s)
		}
		for _, c := range p {
			v[i] = v[i]*10 + int(c-'0')
		}
	}
	d, err := NewDate(v[0], v[1], v[2])
	if err != nil {
		return Date{}, invalidDate(s)
	}
	return d, nil
}

func invalidDate(input string) error {
	return newError(ErrInvalidDate, map[string]any{"input": input}, "invalid date %q", input)
}

// ParseRelativeDate parses today, yesterday, -Nd, -Nw and -Nm relative to the
// given today (the caller supplies the clock; the domain never reads it).
func ParseRelativeDate(s string, today Date) (Date, error) {
	in := strings.ToLower(strings.TrimSpace(s))
	switch in {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDays(-1)
	}
	if len(in) < 3 || in[0] != '-' || !allDigits(in[1:len(in)-1]) || len(in) > 9 {
		return Date{}, invalidDate(s)
	}
	n := 0
	for _, c := range in[1 : len(in)-1] {
		n = n*10 + int(c-'0')
	}
	var (
		d   Date
		err error
	)
	switch in[len(in)-1] {
	case 'd':
		d, err = today.AddDays(-n)
	case 'w':
		d, err = today.AddDays(-7 * n)
	case 'm':
		d, err = today.AddMonths(-n)
	default:
		return Date{}, invalidDate(s)
	}
	if err != nil {
		return Date{}, invalidDate(s)
	}
	return d, nil
}

// ParseDateInput is the single date parser for CLI and TUI input: ISO date,
// the display form of loc, or a relative expression (see ParseRelativeDate).
func ParseDateInput(s string, loc Locale, today Date) (Date, error) {
	in := strings.TrimSpace(s)
	if d, err := ParseDate(in); err == nil {
		return d, nil
	}
	if d, err := ParseLocalDate(in, loc); err == nil {
		return d, nil
	}
	return ParseRelativeDate(in, today)
}
