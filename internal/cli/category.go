package cli

import "github.com/spf13/cobra"

// newCategoryCmd groups the category subcommands.
func newCategoryCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "category", Short: "Add, list, rename, merge or remove categories"}
	cmd.AddCommand(
		newCategoryAddCmd(deps), newCategoryListCmd(deps), newCategoryRenameCmd(deps),
		newCategoryMergeCmd(deps), newCategoryRemoveCmd(deps),
	)
	return cmd
}
