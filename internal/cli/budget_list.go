package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newBudgetListCmd lists every budget.
func newBudgetListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List budgets",
		Args:    noArgs(),
		Example: "  fui budget list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireBudgeting(deps)
			if err != nil {
				return err
			}
			budgets, err := svc.List(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteBudgetList(deps.Stdout, rt.format, deps.Locale(), budgets)
		},
	}
}
