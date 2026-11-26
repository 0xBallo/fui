package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/assetting"
)

func newAssetValueCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "value ASSET VALUE",
		Short:   "Record an asset valuation",
		Args:    exactArgs(2),
		Example: "  fui asset value House 320000 --date 2026-12-31\n  fui asset value rm House 2026-12-31",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetValue(cmd, deps, svc, args[0], args[1])
		},
	}
	cmd.Flags().String("date", "", "valuation date (default: today)")
	cmd.AddCommand(newAssetValueRemoveCmd(deps, svc))
	return cmd
}

func runAssetValue(cmd *cobra.Command, deps *Deps, svc *assetting.Service, assetRef, value string) error {
	if err := requireAssetting(svc); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	date, _ := cmd.Flags().GetString("date")
	ctx := cmd.Context()
	plan, err := svc.PlanValue(ctx, enabled, assetting.ValueRequest{Asset: assetRef, Date: date, Value: value})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := svc.ApplyValue(ctx, plan); err != nil {
			return err
		}
		return writeAssetValuation(deps.Stdout, rt.format, deps.Locale(), rt.color, plan)
	})
}
