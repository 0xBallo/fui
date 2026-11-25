package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/module"
)

// assetCall resolves the shared prerequisites of every asset command.
func assetCall(cmd *cobra.Command, deps *Deps, svc *assetting.Service) (runtimeFlags, module.Set, error) {
	if err := requireAssetting(svc); err != nil {
		return runtimeFlags{}, module.Set{}, err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return runtimeFlags{}, module.Set{}, err
	}
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return runtimeFlags{}, module.Set{}, err
	}
	return rt, enabled, nil
}

func newAssetShowCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Show an asset, its valuations, linked loan, debt and net value",
		Args:  exactArgs(1),
		Example: "  fui asset show House\n" +
			"  fui asset show House --at 2026-12-31 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, enabled, err := assetCall(cmd, deps, svc)
			if err != nil {
				return err
			}
			at, _ := cmd.Flags().GetString("at")
			view, err := svc.Show(cmd.Context(), enabled, args[0], at)
			if err != nil {
				return err
			}
			return writeAssetShow(deps.Stdout, rt.format, deps.Locale(), rt.color, view)
		},
	}
	cmd.Flags().String("at", "", "show values as of this date (default: today)")
	return cmd
}

func newAssetEditCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit NAME",
		Short: "Edit an asset",
		Args:  exactArgs(1),
		Example: "  fui asset edit House --name \"Main House\" --purchase-value 310000\n" +
			"  fui asset edit Car --kind vehicle --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, enabled, err := assetCall(cmd, deps, svc)
			if err != nil {
				return err
			}
			req := assetting.EditRequest{Ref: args[0]}
			for flag, dst := range map[string]**string{
				"name": &req.Name, "kind": &req.Kind, "purchase-date": &req.PurchaseDate,
				"purchase-value": &req.PurchaseValue, "currency": &req.Currency,
			} {
				if !cmd.Flags().Changed(flag) {
					continue
				}
				v, _ := cmd.Flags().GetString(flag)
				*dst = &v
			}
			ctx := cmd.Context()
			plan, err := svc.PlanEdit(ctx, enabled, req)
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				asset, err := svc.ApplyEdit(ctx, plan)
				if err != nil {
					return err
				}
				return writeAsset(deps.Stdout, rt.format, deps.Locale(), rt.color, asset)
			})
		},
	}
	cmd.Flags().String("name", "", "new name")
	cmd.Flags().String("kind", "", "asset kind: property, vehicle or other")
	cmd.Flags().String("purchase-date", "", "purchase date")
	cmd.Flags().String("purchase-value", "", "purchase value")
	cmd.Flags().String("currency", "", "ISO 4217 currency (only while the asset has no valuations)")
	return cmd
}

func newAssetRemoveCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Remove an asset and its valuations",
		Args:    exactArgs(1),
		Example: "  fui asset rm House --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, enabled, err := assetCall(cmd, deps, svc)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanRemove(ctx, enabled, args[0])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyRemove(ctx, plan); err != nil {
					return err
				}
				return writeAssetAction(deps.Stdout, rt.format, assetActionDTO{Action: "removed", Asset: plan.Asset.Name})
			})
		},
	}
}

func newAssetValueRemoveCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME DATE",
		Short:   "Remove an asset valuation",
		Args:    exactArgs(2),
		Example: "  fui asset value rm House 2026-12-31 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, enabled, err := assetCall(cmd, deps, svc)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanRemoveValuation(ctx, enabled, args[0], args[1])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyRemoveValuation(ctx, plan); err != nil {
					return err
				}
				return writeAssetAction(deps.Stdout, rt.format, assetActionDTO{
					Action: "valuation removed from", Asset: plan.Asset.Name, Date: plan.Valuation.Date.String(),
				})
			})
		},
	}
}

func newAssetLinkCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "link NAME LOAN",
		Short:   "Link an asset to the loan that financed it",
		Args:    exactArgs(2),
		Example: "  fui asset link Car \"Car Loan\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, enabled, err := assetCall(cmd, deps, svc)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanLink(ctx, enabled, args[0], args[1])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyLink(ctx, plan); err != nil {
					return err
				}
				return writeAssetAction(deps.Stdout, rt.format, assetActionDTO{
					Action: "linked", Asset: plan.Asset.Name, Loan: plan.Loan.Name,
				})
			})
		},
	}
}

func newAssetUnlinkCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "unlink NAME",
		Short:   "Remove the loan link of an asset",
		Args:    exactArgs(1),
		Example: "  fui asset unlink Car",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, enabled, err := assetCall(cmd, deps, svc)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanUnlink(ctx, enabled, args[0])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyUnlink(ctx, plan); err != nil {
					return err
				}
				return writeAssetAction(deps.Stdout, rt.format, assetActionDTO{Action: "unlinked", Asset: plan.Asset.Name})
			})
		},
	}
}
