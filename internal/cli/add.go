package cli

import "github.com/spf13/cobra"

// newAddCmd is the quick-entry shorthand for adding one transaction: fui add AMOUNT
// DESCRIPTION... It disables cobra's flag parsing; see splitLeadingPositional.
func newAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add AMOUNT [DESCRIPTION...]",
		Short: "Add a transaction",
		Example: "  fui add -12,50 Esselunga -c Groceries -a Checking\n" +
			"  fui add -- -12,50 Cash withdrawal -a Checking",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			amount, rest, help, err := splitLeadingPositional(cmd, args)
			if help {
				return cmd.Help()
			}
			if err != nil {
				return err
			}
			return runAdd(cmd, deps, amount, rest)
		},
	}
	registerAddFlags(cmd)
	return cmd
}

func runAdd(cmd *cobra.Command, deps *Deps, amount string, rest []string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := addTxRequest(cmd, deps, rt, amount, rest)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanAddTx(ctx, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		tx, err := deps.Service.ApplyAddTx(ctx, plan)
		if err != nil {
			return err
		}
		return renderTxByID(ctx, deps, rt, tx.ID)
	})
}
