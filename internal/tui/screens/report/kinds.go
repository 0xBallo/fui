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
	// Filter, GroupBy and Granularity say which options of the filter form the report honours.
	Filter      bool
	GroupBy     bool
	Granularity bool
}

// predefinedKinds are the ad-hoc reports this screen can run directly, in cycling order,
// starting with the reports that need no params so the first load always has something to
// show. They match the CLI's `fui report KIND` list plus forecast; spending and cash flow run
// as grouped reports (category and month by default) and come last so the older kinds keep
// their cycling position.
func predefinedKinds() []kindInfo {
	return []kindInfo{
		{Kind: reporting.KindNetWorth, Label: "Net worth", NeedsPeriod: true, Granularity: true},
		{Kind: reporting.KindBudget, Label: "Budget vs actual", NeedsPeriod: true},
		{Kind: reporting.KindPayees, Label: "Top payees", NeedsLimit: true, NeedsPeriod: true, Filter: true},
		{Kind: reporting.KindForecast, Label: "Forecast", IsForecast: true, Granularity: true},
		{Kind: reporting.KindBalance, Label: "Balance over time", NeedsAccount: true, NeedsPeriod: true, Filter: true, Granularity: true},
		{Kind: reporting.KindDebt, Label: "Remaining debt", NeedsLoan: true, NeedsPeriod: true},
		{Kind: reporting.KindInterest, Label: "Interest paid", NeedsLoan: true, NeedsPeriod: true},
		{Kind: reporting.KindSpending, Label: "Spending by category", NeedsPeriod: true, Filter: true, GroupBy: true},
		{Kind: reporting.KindCashFlow, Label: "Cash flow", NeedsPeriod: true, Filter: true, GroupBy: true},
	}
}
