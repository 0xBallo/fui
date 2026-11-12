package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/cli/render"
)

// newBudgetSetCmd creates or updates a budget covering one category or tag ("#name" for a tag).
// Without --month it replaces the default monthly amount; with --month it overrides that single
// month (the budget must already exist).
func newBudgetSetCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set TARGET AMOUNT",
		Short:   "Set a budget's amount, overall or for one month",
		Args:    exactArgs(2),
		Example: "  fui budget set Groceries 400\n  fui budget set Groceries 500 --month 2026-12 --carry",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireBudgeting(deps)
			if err != nil {
				return err
			}
			month, _ := cmd.Flags().GetString("month")
			name, cats, tags := parseBudgetTarget(args[0])
			ctx := cmd.Context()
			plan, err := svc.PlanSet(ctx, budgeting.SetRequest{
				Name: name, Categories: cats, Tags: tags, Amount: args[1], Month: month,
				Carry: changedBool(cmd, "carry"),
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				b, err := svc.ApplySet(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteBudget(deps.Stdout, rt.format, deps.Locale(), b)
			})
		},
	}
	cmd.Flags().String("month", "", "override the amount for this month only (YYYY-MM)")
	cmd.Flags().Bool("carry", false, "carry the remaining of each period into the next (caps only)")
	return cmd
}
