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
	period, err := reportPeriodValue(cmd, deps.Config.MonthStartDay)
	if err != nil {
		return err
	}
	chart, _ := cmd.Flags().GetBool("chart")
	account, _ := cmd.Flags().GetStringArray("account")
	loan, _ := cmd.Flags().GetString("loan")
	limit, _ := cmd.Flags().GetInt("limit")
	req := reporting.ReportRequest{
		Period: period, Account: firstValue(account), Loan: loan, Limit: limit,
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
		if req.Account == "" {
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
	case "networth":
		out, err := analytics.NetWorth(cmd.Context(), req)
		if err != nil {
			return err
		}
		return render.WriteSeriesReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out, chart)
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

func requireAnalytics(deps *Deps) (*reporting.Analytics, error) {
	if deps == nil || deps.Reporting.Analytics == nil {
		return nil, fmt.Errorf("fui: analytics service is not configured")
	}
	return deps.Reporting.Analytics, nil
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func missingField(name string) error {
	return &domain.Error{
		Code:    domain.CodeMissingField,
		Message: fmt.Sprintf("%s is required", name),
		Details: map[string]any{"field": name},
	}
}
