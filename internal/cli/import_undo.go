package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newImportUndoCmd removes the transactions and batch record of one import.
func newImportUndoCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "undo BATCH",
		Short:   "Undo an import batch",
		Args:    exactArgs(1),
		Example: "  fui import undo 3 --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportUndo(cmd, deps, args[0])
		},
	}
}

func runImportUndo(cmd *cobra.Command, deps *Deps, idArg string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireImporting(deps)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil {
		return missingFieldError("batch")
	}
	ctx := cmd.Context()
	plan, err := svc.PlanUndo(ctx, domain.ImportBatchID(id))
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := svc.ApplyUndo(ctx, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				Removed int `json:"removed_transactions"`
			}{len(plan.Transactions)})
		}
		_, err := fmt.Fprintf(deps.Stdout, "Undone import batch %d (%d transactions removed).\n", id, len(plan.Transactions))
		if err != nil {
			return fmt.Errorf("write undo confirmation: %w", err)
		}
		return nil
	})
}
