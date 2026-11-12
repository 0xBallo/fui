package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newTagMergeCmd retags every transaction tagged SOURCE with TARGET and removes SOURCE.
func newTagMergeCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "merge SOURCE TARGET",
		Short:   "Merge a tag into another",
		Args:    exactArgs(2),
		Example: "  fui tag merge vacanze vacanza --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanMergeTag(ctx, app.MergeTagRequest{
				Source: args[0], Target: args[1],
			})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := deps.Service.ApplyMergeTag(ctx, plan); err != nil {
					return err
				}
				return renderMovedCount(deps, rt, plan.Moved)
			})
		},
	}
}

// renderMovedCount reports how many transactions were reassigned by a tag or payee merge.
func renderMovedCount(deps *Deps, rt runtimeFlags, moved int) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Moved int `json:"moved"`
		}{moved})
	}
	if _, err := fmt.Fprintf(deps.Stdout, "Moved %d transaction(s).\n", moved); err != nil {
		return fmt.Errorf("write moved count: %w", err)
	}
	return nil
}
