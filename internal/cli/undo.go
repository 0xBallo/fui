package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newUndoCmd reverts the most recent undoable write (tx add, edit, rm, transfer, split, categorize,
// tag/untag, rules apply, recurring confirm, loan confirm). It always previews what will change
// and asks for confirmation unless --yes is given.
func newUndoCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "undo",
		Short:   "Undo the last undoable write",
		Long:    "Revert the most recent undoable write: tx add, edit, rm, transfer, split, categorize, tag/untag, rules apply, recurring confirm or loan confirm.\nThe plan is always shown first; warnings list references that were cleared because the row they pointed to no longer exists.",
		Args:    exactArgs(0),
		Example: "  fui undo --dry-run\n  fui undo --yes\n  fui undo --yes --json",
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
	preview := func() error { return writeUndoPlan(deps, rt, confirm, plan.Warnings) }
	return writeChangePreview(cmd, deps, rt, confirm, preview, func() error {
		if err := deps.Service.ApplyUndo(ctx, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				OpID     int64    `json:"op_id"`
				Kind     string   `json:"kind"`
				Reverted int      `json:"reverted"`
				Warnings []string `json:"warnings,omitempty"`
			}{plan.Entry.ID, plan.Entry.Kind, len(plan.Changes), plan.Warnings})
		}
		if err := writeUndoWarnings(deps, plan.Warnings); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(deps.Stdout, "Undone %s (operation %d, %d changes).\n", plan.Entry.Kind, plan.Entry.ID, len(plan.Changes)); err != nil {
			return fmt.Errorf("write undo confirmation: %w", err)
		}
		return nil
	})
}

// writeUndoPlan renders the plan; in JSON the warnings join the plan object, otherwise they go to stderr.
func writeUndoPlan(deps *Deps, rt runtimeFlags, plan app.Plan, warnings []string) error {
	if rt.format != render.JSON || len(warnings) == 0 {
		if err := render.WritePlan(deps.Stdout, rt.format, plan); err != nil {
			return err
		}
		if rt.format == render.JSON {
			return nil
		}
		return writeUndoWarnings(deps, warnings)
	}
	var buf bytes.Buffer
	if err := render.WritePlan(&buf, rt.format, plan); err != nil {
		return err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		return fmt.Errorf("decode undo plan: %w", err)
	}
	raw, err := json.Marshal(warnings)
	if err != nil {
		return fmt.Errorf("encode undo warnings: %w", err)
	}
	obj["warnings"] = raw
	return render.WriteJSON(deps.Stdout, obj)
}

// writeUndoWarnings prints one "warning: ..." line per warning to stderr, like the other commands.
func writeUndoWarnings(deps *Deps, warnings []string) error {
	for _, w := range warnings {
		if _, err := fmt.Fprintf(deps.Stderr, "warning: %s\n", w); err != nil {
			return fmt.Errorf("write undo warning: %w", err)
		}
	}
	return nil
}
