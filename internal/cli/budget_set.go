package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/cli/render"
)

// newBudgetSetCmd creates or updates the budget NAME. Without --category/--tag the budget covers
// the category called NAME ("#name" with no flags covers the tag name). Without --month it
// replaces the default amount; with --month it overrides that single month, creating the budget
// first when it does not exist.
func newBudgetSetCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set NAME AMOUNT",
		Short: "Create a budget or set its amount, overall or for one month",
		Args:  exactArgs(2),
		Example: "  fui budget set Spesa 400 --month 2026-11\n" +
			"  fui budget set Casa 1200 --category Affitto --category Utenze --tag casa\n" +
			"  fui budget set Vacanze 2400 --period year --warn 70 --over 100 --no-carry",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireBudgeting(deps)
			if err != nil {
				return err
			}
			req, err := budgetSetRequest(cmd, args)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanSet(ctx, req)
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
	cmd.Flags().StringArray("category", nil, "category covered by the budget (repeatable)")
	cmd.Flags().StringArray("tag", nil, "tag covered by the budget (repeatable)")
	cmd.Flags().String("period", "", "budget period: month or year (default: month)")
	cmd.Flags().Int("warn", 0, "warning threshold, percent of the amount (default 80)")
	cmd.Flags().Int("over", 0, "over-budget threshold, percent of the amount (default 100)")
	cmd.Flags().Bool("carry", false, "carry the remaining of each period into the next (default for new caps)")
	cmd.Flags().Bool("no-carry", false, "do not carry the remaining into the next period")
	return cmd
}

// budgetSetRequest maps the flags of `budget set` to a budgeting.SetRequest.
func budgetSetRequest(cmd *cobra.Command, args []string) (budgeting.SetRequest, error) {
	f := cmd.Flags()
	if f.Changed("carry") && f.Changed("no-carry") {
		return budgeting.SetRequest{}, newUsageError(errors.New("--carry and --no-carry are mutually exclusive"))
	}
	month, _ := f.GetString("month")
	period, _ := f.GetString("period")
	cats, _ := f.GetStringArray("category")
	tags, _ := f.GetStringArray("tag")
	req := budgeting.SetRequest{
		Name: args[0], Categories: cats, Tags: tags, Amount: args[1], Month: month, Period: period,
		Carry: changedBool(cmd, "carry"),
	}
	if f.Changed("no-carry") {
		off := false
		req.Carry = &off
	}
	req.Warn, req.Over = changedInt(cmd, "warn"), changedInt(cmd, "over")
	if after, ok := strings.CutPrefix(req.Name, "#"); ok && len(cats)+len(tags) == 0 {
		req.Name, req.Tags = after, []string{after}
	}
	return req, nil
}
