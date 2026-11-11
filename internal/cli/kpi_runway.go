package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newKPIRunwayCmd computes how long current liquidity covers reference monthly spending, as of
// --at (default today).
func newKPIRunwayCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runway",
		Short:   "Compute the runway KPI (months of liquidity left at current spending)",
		Example: "  fui kpi runway --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runKPIRunway(cmd, deps)
		},
	}
	cmd.Flags().String("at", "", "evaluate as of this date (default: today)")
	reportAccountFlag(cmd)
	return cmd
}

func runKPIRunway(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	at, _ := cmd.Flags().GetString("at")
	var asOf domain.Date
	if at != "" {
		asOf, err = domain.ParseDate(at)
		if err != nil {
			return err
		}
	}
	names, _ := cmd.Flags().GetStringArray("account")
	accountIDs, err := resolveAccountIDs(ctx, deps, names)
	if err != nil {
		return err
	}
	base, err := deps.BaseCurrency()
	if err != nil {
		return err
	}
	result, err := deps.Reporting.Runway.Run(ctx, reporting.RunwayParams{
		AsOf: asOf, MonthStartDay: deps.Config.MonthStartDay, AccountIDs: accountIDs, BaseCurrency: base,
	})
	if err != nil {
		return err
	}
	return render.WriteRunway(deps.Stdout, rt.format, deps.Locale(), rt.color, result)
}
