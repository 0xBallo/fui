package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newLoanRunCmd generates the pending installment transactions due through --until (today by
// default) for one loan, or for every loan when --loan is omitted.
func newLoanRunCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Short:   "Generate pending installment transactions due up to a date",
		Args:    noArgs(),
		Example: "  fui loan run --until 2025-03-31\n  fui loan run --loan Mutuo",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLoanRun(cmd, deps)
		},
	}
	cmd.Flags().String("loan", "", "only this loan (default: every loan)")
	cmd.Flags().String("until", "", "generate installments due up to this date (default: today)")
	return cmd
}

func runLoanRun(cmd *cobra.Command, deps *Deps) error {
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
	ref, _ := cmd.Flags().GetString("loan")
	until, _ := cmd.Flags().GetString("until")
	ctx := cmd.Context()
	plan, err := deps.Loans.PlanInstallments(ctx, enabled, ref, until)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		created, err := deps.Loans.ApplyInstallments(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteRecurringRunResult(deps.Stdout, rt.format, created)
	})
}
