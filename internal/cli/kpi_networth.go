package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

// newKPINetWorthCmd computes UC-04's net worth: every account balance converted to the base
// currency, plus the total, as of --at (default today).
func newKPINetWorthCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "networth",
		Short:   "Compute the net worth KPI (every account balance converted to the base currency)",
		Example: "  fui kpi networth --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runKPINetWorth(cmd, deps)
		},
	}
	cmd.Flags().String("at", "", "evaluate as of this date (default: today)")
	cmd.Flags().Bool("include-archived", false, "include archived accounts")
	return cmd
}

func runKPINetWorth(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	at, _ := cmd.Flags().GetString("at")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	nw, err := deps.Service.NetWorth(cmd.Context(), app.NetWorthRequest{At: at, IncludeArchived: includeArchived})
	if err != nil {
		return err
	}
	point, err := netWorthPoint(cmd, deps, at)
	if err != nil {
		return err
	}
	return writeKPINetWorth(deps.Stdout, rt.format, deps.Locale(), rt.color, nw, point)
}

func netWorthPoint(cmd *cobra.Command, deps *Deps, at string) (reporting.NetWorthPoint, error) {
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return reporting.NetWorthPoint{}, err
	}
	asOf := at
	if asOf == "" {
		point, err := analytics.NetWorthAt(cmd.Context(), domain.Date{})
		if err != nil {
			return reporting.NetWorthPoint{}, err
		}
		asOf = point.Date
	}
	date, err := domain.ParseDate(asOf)
	if err != nil {
		return reporting.NetWorthPoint{}, err
	}
	period, err := domain.NewPeriod(date, date)
	if err != nil {
		return reporting.NetWorthPoint{}, err
	}
	series, err := analytics.NetWorthReport(cmd.Context(), reporting.ReportRequest{Period: period})
	if err != nil {
		return reporting.NetWorthPoint{}, err
	}
	if len(series.Points) != 1 {
		return reporting.NetWorthPoint{}, fmt.Errorf("net worth report at %s: expected one point, got %d", date, len(series.Points))
	}
	return series.Points[0], nil
}
