package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newBudgetStatusCmd shows spent/planned/remaining/percent (caps) or the envelope ledger plus
// "to assign" (envelopes) for one month, with the spending covered by no budget.
func newBudgetStatusCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show budget status for a month",
		Args:    noArgs(),
		Example: "  fui budget status --month 2026-11",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireBudgeting(deps)
			if err != nil {
				return err
			}
			month, _ := cmd.Flags().GetString("month")
			st, err := svc.Status(cmd.Context(), month)
			if err != nil {
				return err
			}
			return render.WriteBudgetStatus(deps.Stdout, rt.format, deps.Locale(), rt.color, st)
		},
	}
	cmd.Flags().String("month", "", "month to evaluate (YYYY-MM, default: current)")
	return cmd
}
