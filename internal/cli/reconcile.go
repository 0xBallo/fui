package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newReconcileCmd compares one account with a statement balance at a date and closes only when
// the difference is zero.
func newReconcileCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reconcile ACCOUNT",
		Short:   "Reconcile an account against a statement balance",
		Args:    exactArgs(1),
		Example: "  fui reconcile Fineco --balance 2340,18 --date 2026-10-31 --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReconcile(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("balance", "", "statement balance at --date")
	cmd.Flags().String("date", "", "statement date (default: today)")
	return cmd
}

func runReconcile(cmd *cobra.Command, deps *Deps, account string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	balance, _ := cmd.Flags().GetString("balance")
	if balance, err = promptString(deps, rt, "balance", "Statement balance", balance); err != nil {
		return err
	}
	date, _ := cmd.Flags().GetString("date")
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanReconcile(ctx, enabled, app.ReconcileRequest{
		Account: account, StatementBalance: balance, Date: date,
	})
	if err != nil {
		return err
	}
	views, err := reconcileViews(ctx, deps, plan)
	if err != nil {
		return err
	}
	if err := render.WriteReconcile(deps.Stdout, rt.format, deps.Locale(), rt.color, plan, views); err != nil {
		return err
	}
	if rt.dryRun || !plan.Difference.IsZero() {
		return nil
	}
	if !rt.yes {
		return confirmationRequired("reconcile close requires --yes")
	}
	if err := deps.Service.ApplyReconcile(ctx, enabled, plan); err != nil {
		return err
	}
	if rt.format == render.Table {
		if _, err := fmt.Fprintln(deps.Stdout, "Reconciliation closed."); err != nil {
			return fmt.Errorf("write reconciliation confirmation: %w", err)
		}
	}
	return nil
}

func reconcileViews(ctx context.Context, deps *Deps, plan app.ReconcilePlan) ([]app.TxView, error) {
	out := make([]app.TxView, 0, len(plan.Unreconciled))
	for _, tx := range plan.Unreconciled {
		v, err := deps.Service.ShowTx(ctx, tx.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
