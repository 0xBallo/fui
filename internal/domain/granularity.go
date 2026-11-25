package domain

import (
	"strings"
	"time"
)

// Granularity is the bucket size of a time-series report.
type Granularity string

// Supported granularities.
const (
	GranDay   Granularity = "day"
	GranWeek  Granularity = "week"
	GranMonth Granularity = "month"
	GranYear  Granularity = "year"
)

// ParseGranularity parses s case-insensitively.
func ParseGranularity(s string) (Granularity, error) {
	g := Granularity(strings.ToLower(strings.TrimSpace(s)))
	switch g {
	case GranDay, GranWeek, GranMonth, GranYear:
		return g, nil
	}
	return "", invalidValue("granularity", s)
}

// BucketEnds returns the last day of every bucket touching p, the last one clipped to p.To.
// Weeks are ISO weeks ending on Sunday; months are accounting months starting on monthStartDay.
func (g Granularity) BucketEnds(p Period, monthStartDay int) ([]Date, error) {
	var out []Date
	cur := p.From()
	for {
		end, err := g.bucketEnd(cur, monthStartDay)
		if err != nil {
			return nil, err
		}
		if !end.Before(p.To()) {
			return append(out, p.To()), nil
		}
		out = append(out, end)
		if cur, err = end.AddDays(1); err != nil {
			return nil, err
		}
	}
}

// bucketEnd returns the last day of the bucket containing d.
func (g Granularity) bucketEnd(d Date, monthStartDay int) (Date, error) {
	switch g {
	case GranDay:
		return d, nil
	case GranWeek:
		// Weekday: Sunday=0, so days to Sunday is (7-wd)%7.
		wd := int(d.In(time.UTC).Weekday())
		return d.AddDays((7 - wd) % 7)
	case GranMonth:
		mp, err := MonthPeriod(d, monthStartDay)
		if err != nil {
			return Date{}, err
		}
		return mp.To(), nil
	case GranYear:
		return NewDate(d.Year(), 12, 31)
	}
	return Date{}, invalidValue("granularity", string(g))
}
