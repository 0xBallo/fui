package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// NewReportCompareCmd builds `fui report compare`, the standalone form of `fui report run compare`.
func NewReportCompareCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare the spending of two periods",
		Example: `  fui report compare --a last-month --b this-month --by category --json
  fui report compare --a 2026-09 --b 2026-Q4 --category Food --chart`,
		Args: noArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error { return RunReportCompare(cmd, deps) },
	}
	ReportCompareFlags(cmd)
	reportChartFlag(cmd)
	return cmd
}

// ReportCompareFlags registers the --a, --b, --by and transaction filter flags on cmd, for
// `fui report run` and `fui report compare`.
func ReportCompareFlags(cmd *cobra.Command) {
	reportComparePeriodFlags(cmd)
	cmd.Flags().String("by", "", "break the comparison down by category")
	reportFilterFlags(cmd)
}

// reportComparePeriodFlags registers --a and --b, accepted by `report save` too.
func reportComparePeriodFlags(cmd *cobra.Command) {
	cmd.Flags().String("a", "", "first period: relative, YYYY-MM, YYYY-Qn or YYYY (default last-month)")
	cmd.Flags().String("b", "", "second period: relative, YYYY-MM, YYYY-Qn or YYYY (default this-month)")
}

// RunReportCompare runs the compare report from the flags of cmd.
func RunReportCompare(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return err
	}
	a, _ := cmd.Flags().GetString("a")
	b, _ := cmd.Flags().GetString("b")
	by, _ := cmd.Flags().GetString("by")
	if by != "" && by != "category" {
		return &domain.Error{
			Code:    domain.CodeInvalidValue,
			Message: fmt.Sprintf("unknown --by value %q (allowed: category)", by),
			Details: map[string]any{"field": "by", "value": by},
		}
	}
	filter, err := readReportFilter(cmd)
	if err != nil {
		return err
	}
	out, err := analytics.ComparePeriods(cmd.Context(), reporting.CompareRequest{
		A: a, B: b, ByCategory: by == "category", Filter: filter,
	})
	if err != nil {
		return err
	}
	return render.WriteCompareReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out)
}
