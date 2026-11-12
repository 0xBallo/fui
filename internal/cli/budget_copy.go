package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newBudgetCopyCmd copies the previous month's budgets into --to (default: current month): cap
// amount overrides in cap mode, the net pool assignments in envelope mode. --from is optional and
// only checked: the service always copies the month right before --to.
func newBudgetCopyCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy",
		Short:   "Copy last month's budgets into a month",
		Args:    noArgs(),
		Example: "  fui budget copy --from 2026-10 --to 2026-11",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBudgetCopy(cmd, deps)
		},
	}
	cmd.Flags().String("from", "", "source month (YYYY-MM); checked against the month before --to")
	cmd.Flags().String("to", "", "target month (YYYY-MM, default: current)")
	return cmd
}

func runBudgetCopy(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireBudgeting(deps)
	if err != nil {
		return err
	}
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	ctx := cmd.Context()
	plan, err := svc.PlanCopyPrevious(ctx, to)
	if err != nil {
		return err
	}
	if err := checkCopyFrom(from, plan.Period, deps.Config.MonthStartDay); err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := svc.ApplyCopyPrevious(ctx, plan); err != nil {
			return err
		}
		return renderBudgetCopyResult(deps, rt, len(plan.Updates), len(plan.Moves))
	})
}

// checkCopyFrom fails when the caller gave --from and it is not the month right before target:
// PlanCopyPrevious always copies that month, so a mismatched --from would silently do the wrong
// thing otherwise.
func checkCopyFrom(from string, target domain.Period, startDay int) error {
	if from == "" {
		return nil
	}
	prevEnd, err := target.From().AddDays(-1)
	if err != nil {
		return err
	}
	prevPeriod, err := domain.MonthPeriod(prevEnd, startDay)
	if err != nil {
		return err
	}
	fromDate, err := domain.ParseDate(from + "-01")
	if err != nil {
		return fmt.Errorf("from: %w", err)
	}
	fromPeriod, err := domain.MonthPeriod(fromDate, startDay)
	if err != nil {
		return err
	}
	if domain.MonthKeyOf(prevPeriod) == domain.MonthKeyOf(fromPeriod) {
		return nil
	}
	return &domain.Error{
		Code:    domain.CodeInvalidValue,
		Message: fmt.Sprintf("--from %s is not the month right before the target month", from),
		Details: map[string]any{"field": "from", "value": from},
	}
}

func renderBudgetCopyResult(deps *Deps, rt runtimeFlags, updates, moves int) error {
	return render.WriteBudgetCopyResult(deps.Stdout, rt.format, updates, moves)
}
