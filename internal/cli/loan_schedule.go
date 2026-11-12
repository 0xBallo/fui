package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newLoanScheduleCmd shows the installments of the schedule version in force on a date.
func newLoanScheduleCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schedule NAME",
		Short:   "Show a loan's installment schedule",
		Args:    exactArgs(1),
		Example: "  fui loan schedule Mutuo --at 2026-01-01",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoanSchedule(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("at", "", "the schedule version in force on this date (default: today)")
	return cmd
}

func runLoanSchedule(cmd *cobra.Command, deps *Deps, ref string) error {
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
	v, err := deps.Loans.Show(cmd.Context(), enabled, ref, at)
	if err != nil {
		return err
	}
	return render.WriteLoanSchedule(deps.Stdout, rt.format, deps.Locale(), v.Schedule)
}
