package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
)

var analyticsReportKinds = []string{
	"budget-vs-actual",
	"balance",
	"remaining-debt",
	"interest-paid",
	"networth",
	"top-payees",
	"spending",
	"cash-flow",
}

// newReportCmd runs analytics reports by kind, and manages saved reports.
func newReportCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report KIND",
		Short: "Run an analytics report or manage saved reports",
		Example: `  fui report spending --period 2026-Q3 --category Food --group-by payee
  fui report cash-flow --period 2026 --granularity week --json
  fui report balance --account Checking --granularity week
  fui report networth --period 2026 --granularity year`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return newUsageError(fmt.Errorf("accepts 1 arg(s), received %d", len(args)))
			}
			if !slices.Contains(analyticsReportKinds, args[0]) {
				return newUsageError(fmt.Errorf("unknown report kind %q", args[0]))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyticsReportByKind(cmd, deps, args[0])
		},
	}
	reportKindPeriodFlag(cmd)
	reportFilterFlags(cmd)
	reportGroupFlags(cmd)
	reportChartFlag(cmd)
	cmd.Flags().String("loan", "", "filter by loan (required by remaining-debt)")
	cmd.Flags().Int("limit", reporting.DefaultTopPayees, "maximum rows for top-payees")
	cmd.AddCommand(
		newReportSaveCmd(deps),
		newReportListCmd(deps),
		newReportRunSavedCmd(deps),
		newReportRmCmd(deps),
	)
	return cmd
}
