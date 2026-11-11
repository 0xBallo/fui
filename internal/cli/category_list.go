package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newCategoryListCmd lists every category.
func newCategoryListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List categories",
		Example: "  fui category list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			cats, err := deps.Service.ListCategories(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteCategoryList(deps.Stdout, rt.format, cats)
		},
	}
}
