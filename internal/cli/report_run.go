package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

func runAnalyticsReportByKind(cmd *cobra.Command, deps *Deps, kind string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return err
	}
	period, err := reportPeriodValue(cmd, deps)
	if err != nil {
		return err
	}
	filter, err := readReportFilter(cmd)
	if err != nil {
		return err
	}
	group, gran, err := reportGrouping(cmd, kind)
	if err != nil {
		return err
	}
	chart, _ := cmd.Flags().GetBool("chart")
	loan, _ := cmd.Flags().GetString("loan")
	limit, _ := cmd.Flags().GetInt("limit")
	req := reporting.ReportRequest{
		Period: period, Loan: loan, Limit: limit, Filter: filter, GroupBy: group, Granularity: gran,
	}
	return executeAnalyticsReport(cmd, deps, rt, analytics, kind, req, chart)
}

func executeAnalyticsReport(
	cmd *cobra.Command,
	deps *Deps,
	rt runtimeFlags,
	analytics *reporting.Analytics,
	kind string,
	req reporting.ReportRequest,
	chart bool,
) error {
	switch kind {
	case "budget-vs-actual":
		out, err := analytics.BudgetVsActual(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WriteBudgetReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
	case "balance":
		if len(req.Filter.Accounts) == 0 {
			return missingField("account")
		}
		out, err := analytics.BalanceOverTime(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WriteSeriesReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
	case "remaining-debt":
		if req.Loan == "" {
			return missingField("loan")
		}
		out, err := analytics.RemainingDebt(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WriteDebtReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
	case "interest-paid":
		out, err := analytics.InterestPaid(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WriteInterestReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
	case "networth", "spending", "cash-flow":
		return executeSeriesReport(cmd, deps, rt, analytics, kind, req, chart)
	case "top-payees":
		out, err := analytics.TopPayees(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WritePayeesReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
	default:
		return &domain.Error{
			Code:    domain.CodeInvalidValue,
			Message: fmt.Sprintf("unknown report kind %q", kind),
			Details: map[string]any{"field": "kind", "value": kind},
		}
	}
}

// executeSeriesReport runs the reports built over the transaction rows or net worth series.
func executeSeriesReport(
	cmd *cobra.Command,
	deps *Deps,
	rt runtimeFlags,
	analytics *reporting.Analytics,
	kind string,
	req reporting.ReportRequest,
	chart bool,
) error {
	if kind == "networth" {
		out, err := analytics.NetWorthReport(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WriteNetWorthReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
	}
	run := analytics.Spending
	if kind == "cash-flow" {
		run = analytics.CashFlow
	}
	out, err := run(cmd.Context(), req)
	if err != nil {
		return err
	}
	return render.WriteGroupedReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
}

func requireAnalytics(deps *Deps) (*reporting.Analytics, error) {
	if deps == nil || deps.Reporting.Analytics == nil {
		return nil, fmt.Errorf("fui: analytics service is not configured")
	}
	return deps.Reporting.Analytics, nil
}

func missingField(name string) error {
	return &domain.Error{
		Code:    domain.CodeMissingField,
		Message: fmt.Sprintf("%s is required", name),
		Details: map[string]any{"field": name},
	}
}
