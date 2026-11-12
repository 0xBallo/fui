package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newLoanShowCmd shows one loan's overview, accounts and events.
func newLoanShowCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show NAME",
		Short:   "Show a loan's overview, accounts and events",
		Args:    exactArgs(1),
		Example: "  fui loan show Mutuo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoanShow(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("at", "", "the schedule version in force on this date (default: today)")
	return cmd
}

func runLoanShow(cmd *cobra.Command, deps *Deps, ref string) error {
	if err := requireLoans(deps); err != nil {
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
	at, _ := cmd.Flags().GetString("at")
	ctx := cmd.Context()
	v, err := deps.Loans.Show(ctx, enabled, ref, at)
	if err != nil {
		return err
	}
	view, err := buildLoanView(ctx, deps, v.Info, v.Events)
	if err != nil {
		return err
	}
	return render.WriteLoan(deps.Stdout, rt.format, deps.Locale(), view)
}
