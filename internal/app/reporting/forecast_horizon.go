package reporting

import (
	"strconv"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/domain"
)

// DefaultHorizon is the forecast horizon when neither the request nor the config sets one.
const DefaultHorizon = "6m"

// Forecast granularities: one point per day or one per ISO week.
const (
	GranularityDay  = "day"
	GranularityWeek = "week"
)

// Horizon is a forecast span: Count days ('d'), weeks ('w') or calendar months ('m').
type Horizon struct {
	Count int
	Unit  rune
}

// ParseHorizon parses "Nd", "Nw", "Nm" or a bare number (days). Zero, negative and malformed
// values fail with invalid_value.
func ParseHorizon(s string) (Horizon, error) {
	in := strings.TrimSpace(s)
	unit := 'd'
	digits := in
	if in != "" {
		switch last := in[len(in)-1]; last {
		case 'd', 'w', 'm':
			unit, digits = rune(last), in[:len(in)-1]
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 || digits[0] == '+' {
		return Horizon{}, invalidHorizon(s)
	}
	return Horizon{Count: n, Unit: unit}, nil
}

// Days is the number of days between from and the end of the horizon; months are added by
// calendar, so "1m" from 31 January spans to 28/29 February.
func (h Horizon) Days(from domain.Date) (int, error) {
	switch h.Unit {
	case 'd':
		return h.Count, nil
	case 'w':
		return h.Count * 7, nil
	case 'm':
		end, err := from.AddMonths(h.Count)
		if err != nil {
			return 0, err
		}
		return int(end.In(time.UTC).Sub(from.In(time.UTC)).Hours() / 24), nil
	}
	return 0, invalidHorizon(string(h.Unit))
}

func invalidHorizon(v string) error {
	return &domain.Error{
		Code:    domain.CodeInvalidValue,
		Message: "invalid horizon \"" + v + "\" (expected Nd, Nw, Nm or a number of days)",
		Details: map[string]any{"field": "horizon", "value": v},
	}
}

// weekBuckets reduces a daily series to one point per ISO week: the last day of the week
// (Sunday), the last bucket being clipped at the end of the series.
func weekBuckets[T any](dates []domain.Date, vals []T) ([]domain.Date, []T) {
	var outD []domain.Date
	var outV []T
	for i, d := range dates {
		if i == len(dates)-1 || d.In(time.UTC).Weekday() == time.Sunday {
			outD = append(outD, d)
			outV = append(outV, vals[i])
		}
	}
	return outD, outV
}
