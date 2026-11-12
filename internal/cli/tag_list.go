package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newTagListCmd lists every tag.
func newTagListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List tags",
		Args:    noArgs(),
		Example: "  fui tag list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			tags, err := deps.Service.ListTags(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteTagList(deps.Stdout, rt.format, tags)
		},
	}
}
