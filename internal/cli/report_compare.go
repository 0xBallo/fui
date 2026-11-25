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
		Use:     "compare",
		Short:   "Compare the spending of two periods",
		Example: "  fui report compare --a last-month --b this-month --by category --json",
		Args:    noArgs(),
		RunE:    func(cmd *cobra.Command, _ []string) error { return RunReportCompare(cmd, deps) },
	}
	ReportCompareFlags(cmd)
	return cmd
}

// ReportCompareFlags registers the --a, --b and --by flags on cmd, for `fui report run`.
func ReportCompareFlags(cmd *cobra.Command) {
	cmd.Flags().String("a", "", "first period, e.g. last-month (default last-month)")
	cmd.Flags().String("b", "", "second period, e.g. this-month (default this-month)")
	cmd.Flags().String("by", "", "break the comparison down by category")
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
	out, err := analytics.ComparePeriods(cmd.Context(), reporting.CompareRequest{A: a, B: b, ByCategory: by == "category"})
	if err != nil {
		return err
	}
	return render.WriteCompareReport(deps.Stdout, rt.format, deps.Locale(), rt.color, out)
}
