package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newTagRemoveCmd removes a tag, detaching it from every transaction that carries it.
func newTagRemoveCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Remove a tag",
		Args:    exactArgs(1),
		Example: "  fui tag rm vacanze-2026 --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := deps.Service.PlanRemoveTag(ctx, app.RemoveTagRequest{Tag: args[0]})
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := deps.Service.ApplyRemoveTag(ctx, plan); err != nil {
					return err
				}
				return renderDetachedCount(deps, rt, plan.Detached)
			})
		},
	}
}

// renderDetachedCount reports how many transactions were detached by a tag rm.
func renderDetachedCount(deps *Deps, rt runtimeFlags, detached int) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Detached int `json:"detached"`
		}{detached})
	}
	if _, err := fmt.Fprintf(deps.Stdout, "Detached %d transaction(s).\n", detached); err != nil {
		return fmt.Errorf("write detached count: %w", err)
	}
	return nil
}
