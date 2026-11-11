package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newTxRemoveCmd removes a transaction; removing one leg of a transfer removes both legs.
func newTxRemoveCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm ID",
		Short:   "Remove a transaction",
		Args:    cobra.ExactArgs(1),
		Example: "  fui tx rm 42",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxRemove(cmd, deps, args[0])
		},
	}
	cmd.Flags().Bool("confirm-reconciled", false, "allow removing a reconciled transaction")
	return cmd
}

func runTxRemove(cmd *cobra.Command, deps *Deps, idArg string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil {
		return missingFieldError("id")
	}
	confirmReconciled, _ := cmd.Flags().GetBool("confirm-reconciled")
	ctx := cmd.Context()
	plan, err := deps.Service.PlanRemoveTx(ctx, app.RemoveTxRequest{
		ID: domain.TransactionID(id), ConfirmReconciled: confirmReconciled,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Service.ApplyRemoveTx(ctx, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				Removed int `json:"removed"`
			}{len(plan.TxIDs)})
		}
		_, err := fmt.Fprintf(deps.Stdout, "Removed %d transaction(s).\n", len(plan.TxIDs))
		if err != nil {
			return fmt.Errorf("write removed-count note: %w", err)
		}
		return nil
	})
}
