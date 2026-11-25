package simulating

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// Cash-flow event kinds accepted in CashFlowEvent.Kind.
const (
	EventOneOff          = "one_off"
	EventRecurringChange = "recurring_change"
	EventStopIncome      = "stop_income"
	EventNewLoan         = "new_loan"
)

// eventParts is the number of colon separated parts of each event kind's textual form.
var eventParts = map[string]int{
	EventOneOff:          2, // DATE:AMOUNT
	EventRecurringChange: 2, // DATE:AMOUNT
	EventStopIncome:      1, // DATE
	EventNewLoan:         4, // DATE:CAPITAL:RATE:YEARS
}

// ParseEvent parses the textual form of one cash-flow event shared by the CLI flags and the TUI
// form: DATE for stop_income, DATE:AMOUNT for one_off and recurring_change, DATE:CAPITAL:RATE:YEARS
// for new_loan. Amounts use loc and cur; the rate is a percentage. The date is validated here so
// that every front end rejects bad input the same way.
func ParseEvent(kind, entry string, loc domain.Locale, cur domain.Currency) (CashFlowEvent, error) {
	parts, ok := eventParts[kind]
	if !ok {
		return CashFlowEvent{}, invalid("event.kind", fmt.Sprintf("unknown event kind %q", kind))
	}
	bad := func(cause error) error {
		msg := fmt.Sprintf("invalid %s event %q", kind, entry)
		if cause != nil {
			msg += ": " + cause.Error()
		}
		return invalid(kind, msg)
	}
	fs := strings.Split(strings.TrimSpace(entry), ":")
	if len(fs) != parts {
		return CashFlowEvent{}, bad(nil)
	}
	ev := CashFlowEvent{Kind: kind, Date: strings.TrimSpace(fs[0])}
	if _, err := domain.ParseDate(ev.Date); err != nil {
		return ev, bad(err)
	}
	if parts > 1 {
		money, err := domain.ParseAmount(fs[1], loc, cur)
		if err != nil {
			return ev, bad(err)
		}
		ev.AmountMinor = money.Minor()
	}
	if parts == 4 {
		bp, err := ParsePercentBP(fs[2])
		if err != nil {
			return ev, bad(err)
		}
		years, err := strconv.Atoi(strings.TrimSpace(fs[3]))
		if err != nil || years <= 0 {
			return ev, bad(nil)
		}
		ev.RateBP, ev.Years = bp, years
	}
	return ev, nil
}

// ParsePercentBP parses a percentage with up to two decimals ("4,5", "4.25%", "-1") into basis points.
func ParsePercentBP(in string) (int64, error) {
	s := strings.ReplaceAll(strings.TrimSuffix(strings.TrimSpace(in), "%"), ",", ".")
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" && frac == "" || len(frac) > 2 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	if whole == "" {
		whole = "0"
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseInt(whole, 10, 32)
	if err != nil || w < 0 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	f, err := strconv.ParseInt(frac, 10, 32)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	bp := w*100 + f
	if neg {
		bp = -bp
	}
	return bp, nil
}
