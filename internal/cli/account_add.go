package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
)

// newAccountAddCmd creates an account.
func newAccountAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add NAME",
		Short:   "Add an account",
		Args:    exactArgs(1),
		Example: "  fui account add Checking --kind checking --currency EUR --opening-balance 1000",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccountAdd(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("description", "", "description")
	cmd.Flags().String("kind", "", "cash, checking, credit_card, savings, investment or loan")
	cmd.Flags().String("currency", "", "ISO 4217 currency (required)")
	cmd.Flags().String("opening-balance", "", "balance on opened-on (default: 0)")
	cmd.Flags().String("opened-on", "", "date the account was opened (default: today)")
	return cmd
}

func runAccountAdd(cmd *cobra.Command, deps *Deps, name string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	kind, _ := cmd.Flags().GetString("kind")
	if kind, err = promptString(deps, rt, "kind", "Kind", kind); err != nil {
		return err
	}
	currency, _ := cmd.Flags().GetString("currency")
	if currency, err = promptString(deps, rt, "currency", "Currency", currency); err != nil {
		return err
	}
	description, _ := cmd.Flags().GetString("description")
	openingBalance, _ := cmd.Flags().GetString("opening-balance")
	openedOn, _ := cmd.Flags().GetString("opened-on")
	ctx := cmd.Context()
	plan, err := deps.Service.PlanAddAccount(ctx, app.AddAccountRequest{
		Name: name, Description: description, Kind: kind, Currency: currency,
		OpeningBalance: openingBalance, OpenedOn: openedOn,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		acct, err := deps.Service.ApplyAddAccount(ctx, plan)
		if err != nil {
			return err
		}
		return renderAccountByName(cmd, deps, rt, acct.Name)
	})
}
