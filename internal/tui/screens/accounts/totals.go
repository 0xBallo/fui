package accounts

import (
	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// totals holds the rendered total line under each section.
type totals struct{ wealth, loans string }

const totalNA = "Total n/a"

// sectionTotals sums the base-currency values of the net-worth lines per section; a nil net worth
// (no base currency or missing rate) yields "Total n/a".
func sectionTotals(nw *app.NetWorth, loc domain.Locale) totals {
	if nw == nil {
		return totals{wealth: totalNA, loans: totalNA}
	}
	zero, err := domain.NewMoney(0, nw.Base)
	if err != nil {
		return totals{wealth: totalNA, loans: totalNA}
	}
	wealth, loans := zero, zero
	for _, l := range nw.Lines {
		target := &wealth
		if l.Account.Kind == domain.AccountLoan {
			target = &loans
		}
		sum, err := target.Add(l.Converted)
		if err != nil {
			return totals{wealth: totalNA, loans: totalNA}
		}
		*target = sum
	}
	return totals{
		wealth: "Total " + domain.FormatAmount(wealth, loc) + " " + nw.Base.Code(),
		loans:  "Total " + domain.FormatAmount(loans, loc) + " " + nw.Base.Code(),
	}
}
