package cli

import "github.com/spf13/cobra"

// newTagCmd groups the tag subcommands. Not yet registered in the command tree (see V32).
func newTagCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "tag", Short: "Add, list, rename, merge or remove tags"}
	cmd.AddCommand(
		newTagAddCmd(deps), newTagListCmd(deps), newTagRenameCmd(deps),
		newTagMergeCmd(deps), newTagRemoveCmd(deps),
	)
	return cmd
}

// minArgs wraps cobra.MinimumNArgs(n) so too few arguments is a usageError, classified as
// ExitUserError, the same way exactArgs wraps cobra.ExactArgs.
func minArgs(n int) cobra.PositionalArgs {
	validate := cobra.MinimumNArgs(n)
	return func(cmd *cobra.Command, args []string) error {
		return newUsageError(validate(cmd, args))
	}
}
