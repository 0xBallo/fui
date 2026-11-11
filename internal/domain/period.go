package domain

// Period is a closed date interval: both From and To belong to it.
type Period struct {
	from Date
	to   Date
}

// NewPeriod returns the closed interval [from, to]; to must not precede from.
func NewPeriod(from, to Date) (Period, error) {
	if from.IsZero() || to.IsZero() {
		return Period{}, missingField("period")
	}
	if to.Before(from) {
		return Period{}, newError(ErrInvalidDate, map[string]any{"from": from.String(), "to": to.String()},
			"period ends (%s) before it starts (%s)", to, from)
	}
	return Period{from: from, to: to}, nil
}

// From returns the first day of the period.
func (p Period) From() Date { return p.from }

// To returns the last day of the period.
func (p Period) To() Date { return p.to }

// Contains reports whether d lies within the period.
func (p Period) Contains(d Date) bool { return !d.Before(p.from) && !d.After(p.to) }

// MonthPeriod returns the accounting month containing d when months start on
// startDay (1..31). A startDay beyond the length of a month is clamped to that
// month's last day, so 31 means "month end to month end".
func MonthPeriod(d Date, startDay int) (Period, error) {
	if startDay < 1 || startDay > 31 {
		return Period{}, newError(ErrInvalidValue, map[string]any{"field": "month_start_day", "value": startDay},
			"month start day must be 1..31, got %d", startDay)
	}
	cur, err := monthStart(d.year, d.month, startDay)
	if err != nil {
		return Period{}, err
	}
	from, next := cur, cur
	if d.Before(cur) {
		from, err = monthStartAt(d, -1, startDay)
	} else {
		next, err = monthStartAt(d, 1, startDay)
	}
	if err != nil {
		return Period{}, err
	}
	end, err := next.AddDays(-1)
	if err != nil {
		return Period{}, err
	}
	return NewPeriod(from, end)
}

// monthStartAt returns the (clamped) start day of the month n months from d's month.
func monthStartAt(d Date, n, startDay int) (Date, error) {
	shifted, err := NewDate(d.year, d.month, 1)
	if err != nil {
		return Date{}, err
	}
	if shifted, err = shifted.AddMonths(n); err != nil {
		return Date{}, err
	}
	return monthStart(shifted.year, shifted.month, startDay)
}

func monthStart(y, m, startDay int) (Date, error) {
	return NewDate(y, m, min(startDay, daysIn(y, m)))
}
