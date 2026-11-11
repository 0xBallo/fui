package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newFXSetCmd sets the rate of a currency on a date (replacing it when already set).
func newFXSetCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set CURRENCY RATE",
		Short:   "Set a manual exchange rate",
		Args:    exactArgs(2),
		Example: "  fui fx set USD 1.0842 --date 2026-01-01",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			date, _ := cmd.Flags().GetString("date")
			ctx := cmd.Context()
			plan, err := deps.Service.PlanSetFXRate(ctx, app.SetFXRateRequest{
				Currency: args[0], Rate: args[1], Date: date,
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				r, err := deps.Service.ApplySetFXRate(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteFXRate(deps.Stdout, rt.format, r)
			})
		},
	}
	cmd.Flags().String("date", "", "date the rate applies from (default: today)")
	return cmd
}
