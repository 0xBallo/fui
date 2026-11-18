package domain

// Cycle is the kind of period a report or dashboard is calculated over.
type Cycle string

// Supported calculation cycles.
const (
	CycleMonth    Cycle = "month"
	CycleQuarter  Cycle = "quarter"
	CycleYear     Cycle = "year"
	CycleLifetime Cycle = "lifetime"
)

// ParseCycle converts s into a Cycle.
func ParseCycle(s string) (Cycle, error) {
	switch c := Cycle(s); c {
	case CycleMonth, CycleQuarter, CycleYear, CycleLifetime:
		return c, nil
	}
	return "", invalidValue("cycle", s)
}

// Next returns the following cycle kind: month, quarter, year, lifetime, then month again.
func (c Cycle) Next() Cycle {
	switch c {
	case CycleMonth:
		return CycleQuarter
	case CycleQuarter:
		return CycleYear
	case CycleYear:
		return CycleLifetime
	case CycleLifetime:
		return CycleMonth
	}
	return CycleMonth
}

// CyclePeriod returns the period of kind c containing anchor. Months follow
// MonthPeriod(anchor, startDay); quarters and years are calendar based;
// lifetime spans [since, anchor], or just anchor when since is zero.
func CyclePeriod(c Cycle, anchor Date, startDay int, since Date) (Period, error) {
	switch c {
	case CycleMonth:
		return MonthPeriod(anchor, startDay)
	case CycleQuarter:
		first := (anchor.month-1)/3*3 + 1
		return calendarPeriod(anchor.year, first, 3)
	case CycleYear:
		return calendarPeriod(anchor.year, 1, 12)
	case CycleLifetime:
		if since.IsZero() {
			since = anchor
		}
		return NewPeriod(since, anchor)
	}
	return Period{}, invalidValue("cycle", string(c))
}

func calendarPeriod(y, firstMonth, months int) (Period, error) {
	from, err := NewDate(y, firstMonth, 1)
	if err != nil {
		return Period{}, err
	}
	next, err := from.AddMonths(months)
	if err != nil {
		return Period{}, err
	}
	end, err := next.AddDays(-1)
	if err != nil {
		return Period{}, err
	}
	return NewPeriod(from, end)
}

// ShiftCycle moves anchor by n periods of kind c (n < 0 goes back) and returns
// the first day of the target period. Lifetime returns anchor unchanged.
func ShiftCycle(c Cycle, anchor Date, n, startDay int) (Date, error) {
	switch c {
	case CycleMonth:
		cur, err := MonthPeriod(anchor, startDay)
		if err != nil {
			return Date{}, err
		}
		return monthStartAt(cur.From(), n, startDay)
	case CycleQuarter, CycleYear:
		p, err := CyclePeriod(c, anchor, startDay, Date{})
		if err != nil {
			return Date{}, err
		}
		if c == CycleQuarter {
			return p.From().AddMonths(3 * n)
		}
		return p.From().AddMonths(12 * n)
	case CycleLifetime:
		return anchor, nil
	}
	return Date{}, invalidValue("cycle", string(c))
}
