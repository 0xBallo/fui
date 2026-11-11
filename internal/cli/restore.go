package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newRestoreCmd replaces the archive with a backup file; it is always a bulk write (PlanRestore
// sets Plan.Bulk), and ApplyRestore takes a safety backup of the current archive first.
func newRestoreCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "restore PATH",
		Short:   "Restore the archive from a backup",
		Args:    cobra.ExactArgs(1),
		Example: "  fui restore ~/.local/state/fui/backups/fui-2026-01-01.db --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			plan, err := deps.Service.PlanRestore(app.RestoreRequest{Path: args[0]})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := deps.Service.ApplyRestore(cmd.Context(), plan); err != nil {
					return err
				}
				if rt.format == render.JSON {
					return render.WriteJSON(deps.Stdout, struct {
						Restored string `json:"restored"`
					}{plan.Path})
				}
				_, err := fmt.Fprintf(deps.Stdout, "Restored from %s.\n", plan.Path)
				if err != nil {
					return fmt.Errorf("write restore confirmation: %w", err)
				}
				return nil
			})
		},
	}
}
