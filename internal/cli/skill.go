package cli

import "github.com/spf13/cobra"

// newSkillCmd groups the agent-skill subcommands.
func newSkillCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "skill", Short: "Show or install the agent skill"}
	cmd.AddCommand(newSkillShowCmd(deps), newSkillInstallCmd(deps))
	return cmd
}
