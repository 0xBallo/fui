package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newUndoCmd reverts the most recent undoable write (tx add, edit, rm, transfer, split). It
// always previews what will change and asks for confirmation unless --yes is given.
func newUndoCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "undo",
		Short:   "Undo the last transaction write",
		Args:    exactArgs(0),
		Example: "  fui undo --dry-run\n  fui undo --yes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUndo(cmd, deps)
		},
	}
}

func runUndo(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanUndo(ctx)
	if err != nil {
		return err
	}
	// Undo is never silent: ask for confirmation even though the plan itself is not bulk.
	confirm := plan.Plan
	confirm.Bulk = true
	return writeChange(cmd, deps, rt, confirm, func() error {
		if err := deps.Service.ApplyUndo(ctx, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				OpID     int64  `json:"op_id"`
				Kind     string `json:"kind"`
				Reverted int    `json:"reverted"`
			}{plan.Entry.ID, plan.Entry.Kind, len(plan.Changes)})
		}
		if _, err := fmt.Fprintf(deps.Stdout, "Undone %s (operation %d, %d changes).\n", plan.Entry.Kind, plan.Entry.ID, len(plan.Changes)); err != nil {
			return fmt.Errorf("write undo confirmation: %w", err)
		}
		return nil
	})
}
