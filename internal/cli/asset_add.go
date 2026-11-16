package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/assetting"
)

func newAssetAddCmd(deps *Deps, svc *assetting.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "Add an asset",
		Args:  exactArgs(1),
		Example: "  fui asset add House --kind property --value 300000 --purchase-date 2020-01-01\n" +
			"  fui asset add Car --kind vehicle --value 15000 --currency EUR --loan Car Loan",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetAdd(cmd, deps, svc, args[0])
		},
	}
	cmd.Flags().String("kind", "", "asset kind: property, vehicle or other")
	cmd.Flags().String("value", "", "purchase value")
	cmd.Flags().String("purchase-date", "", "purchase date (default: today)")
	cmd.Flags().String("currency", "", "ISO 4217 currency (default: loan currency or base currency)")
	cmd.Flags().String("loan", "", "optional linked loan name")
	return cmd
}

func runAssetAdd(cmd *cobra.Command, deps *Deps, svc *assetting.Service, name string) error {
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
	kind, _ := cmd.Flags().GetString("kind")
	value, _ := cmd.Flags().GetString("value")
	purchaseDate, _ := cmd.Flags().GetString("purchase-date")
	currency, _ := cmd.Flags().GetString("currency")
	loan, _ := cmd.Flags().GetString("loan")
	ctx := cmd.Context()
	plan, err := svc.PlanAdd(ctx, enabled, assetting.AddRequest{
		Name: name, Kind: kind, Value: value, PurchaseDate: purchaseDate, Currency: currency, Loan: loan,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		asset, err := svc.ApplyAdd(ctx, plan)
		if err != nil {
			return err
		}
		return writeAsset(deps.Stdout, rt.format, deps.Locale(), rt.color, asset)
	})
}
