package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCompletionCmd generates a shell completion script for bash, zsh or fish.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion {bash|zsh|fish}",
		Short: "Generate a shell completion script",
		Example: "  fui completion bash > /etc/bash_completion.d/fui\n" +
			"  fui completion zsh > \"${fpath[1]}/_fui\"",
	}
	cmd.AddCommand(
		newCompletionShellCmd("bash", "  fui completion bash > /etc/bash_completion.d/fui",
			func(root *cobra.Command, w *cobra.Command) error {
				return root.GenBashCompletionV2(w.OutOrStdout(), true)
			}),
		newCompletionShellCmd("zsh", "  fui completion zsh > \"${fpath[1]}/_fui\"",
			func(root *cobra.Command, w *cobra.Command) error { return root.GenZshCompletion(w.OutOrStdout()) }),
		newCompletionShellCmd("fish", "  fui completion fish > ~/.config/fish/completions/fui.fish",
			func(root *cobra.Command, w *cobra.Command) error {
				return root.GenFishCompletion(w.OutOrStdout(), true)
			}),
	)
	return cmd
}

// newCompletionShellCmd builds one "fui completion <shell>" leaf command.
func newCompletionShellCmd(shell, example string, gen func(root, cmd *cobra.Command) error) *cobra.Command {
	return &cobra.Command{
		Use:     shell,
		Short:   "Generate a " + shell + " completion script",
		Example: example,
		Args:    noArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := gen(cmd.Root(), cmd); err != nil {
				return fmt.Errorf("generate %s completion: %w", shell, err)
			}
			return nil
		},
	}
}
