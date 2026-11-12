package report

import "github.com/0xballo/fui/internal/app/reporting"

// kindInfo describes one predefined report: its saved-report kind (reporting.KindXxx), a label
// for the title, and which params form fields it needs.
type kindInfo struct {
	Kind         string
	Label        string
	NeedsAccount bool
	NeedsLoan    bool
	NeedsLimit   bool
	NeedsPeriod  bool
	IsForecast   bool
}

// predefinedKinds are the ad-hoc reports this screen can run directly, in cycling order,
// starting with the reports that need no params so the first load always has something to
// show. They match the CLI's `fui report KIND` list plus forecast; spending and cash-flow (MVP
// reports) can only be run as saved reports, since they need the reporting.Source the CLI itself
// requires a saved report wrapper for.
func predefinedKinds() []kindInfo {
	return []kindInfo{
		{Kind: reporting.KindNetWorth, Label: "Net worth", NeedsPeriod: true},
		{Kind: reporting.KindBudget, Label: "Budget vs actual", NeedsPeriod: true},
		{Kind: reporting.KindPayees, Label: "Top payees", NeedsLimit: true, NeedsPeriod: true},
		{Kind: reporting.KindForecast, Label: "Forecast", IsForecast: true},
		{Kind: reporting.KindBalance, Label: "Balance over time", NeedsAccount: true, NeedsPeriod: true},
		{Kind: reporting.KindDebt, Label: "Remaining debt", NeedsLoan: true, NeedsPeriod: true},
		{Kind: reporting.KindInterest, Label: "Interest paid", NeedsLoan: true, NeedsPeriod: true},
	}
}
