package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
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
	cmd.Flags().StringSlice("tick", nil, "mark transaction ids as matching the statement (repeatable, comma list)")
	cmd.Flags().StringSlice("untick", nil, "remove transaction ids from the ticked set (repeatable, comma list)")
	return cmd
}

func parseTickIDs(cmd *cobra.Command, name string) ([]domain.TransactionID, error) {
	raw, _ := cmd.Flags().GetStringSlice(name)
	ids := make([]domain.TransactionID, 0, len(raw))
	for _, s := range raw {
		id, err := parseTxID(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
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
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	req, err := reconcileRequest(cmd, account, balance)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanReconcile(ctx, enabled, req)
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
	if rt.dryRun {
		return nil
	}
	return applyReconcile(ctx, deps, rt, enabled, plan)
}

func reconcileRequest(cmd *cobra.Command, account, balance string) (app.ReconcileRequest, error) {
	date, _ := cmd.Flags().GetString("date")
	tick, err := parseTickIDs(cmd, "tick")
	if err != nil {
		return app.ReconcileRequest{}, err
	}
	untick, err := parseTickIDs(cmd, "untick")
	if err != nil {
		return app.ReconcileRequest{}, err
	}
	return app.ReconcileRequest{Account: account, StatementBalance: balance, Date: date, Tick: tick, Untick: untick}, nil
}

// applyReconcile stores ticks without confirmation (not a close) and closes only on a zero
// difference with --yes.
func applyReconcile(ctx context.Context, deps *Deps, rt runtimeFlags, enabled module.Set, plan app.ReconcilePlan) error {
	if plan.Ticking && !plan.Closing {
		if err := deps.Service.ApplyReconcile(ctx, enabled, plan); err != nil {
			return err
		}
		return writeReconcileNote(deps, rt, "Ticks saved.")
	}
	if !plan.Difference.IsZero() {
		return nil
	}
	if !rt.yes {
		return confirmationRequired("reconcile close requires --yes")
	}
	if err := deps.Service.ApplyReconcile(ctx, enabled, plan); err != nil {
		return err
	}
	return writeReconcileNote(deps, rt, "Reconciliation closed.")
}

func writeReconcileNote(deps *Deps, rt runtimeFlags, note string) error {
	if rt.format != render.Table {
		return nil
	}
	if _, err := fmt.Fprintln(deps.Stdout, note); err != nil {
		return fmt.Errorf("write reconciliation confirmation: %w", err)
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
