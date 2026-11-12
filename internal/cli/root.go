package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// NewRoot builds the fui command tree. With no subcommand it starts the TUI via deps.RunTUI.
func NewRoot(deps *Deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "fui",
		Short:         "Local-first personal finance CLI and TUI",
		Long:          "fui is a local-first personal finance tool: a CLI for scripts and agents, a TUI for everyday use, one SQLite archive.",
		Example:       "  fui init --currency EUR\n  fui add -12,50 Esselunga -c Groceries -a Checking\n  fui account list --json",
		Version:       deps.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return newUsageError(fmt.Errorf("unknown command %q for %q", args[0], "fui"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deps.RunTUI == nil {
				return fmt.Errorf("fui: no TUI available in this build")
			}
			return deps.RunTUI(cmd.Context())
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return newUsageError(err) })
	registerPersistentFlags(root)
	root.AddCommand(
		newInitCmd(deps), newAddCmd(deps), newTxCmd(deps), newTransferCmd(deps),
		newAccountCmd(deps), newCategoryCmd(deps), newFXCmd(deps),
		newBackupCmd(deps), newRestoreCmd(deps), newResetCmd(deps), newConfigCmd(deps), newHelpJSONCmd(deps),
		newReportCmd(deps), newKPICmd(deps), newCompletionCmd(),
		newImportCmd(deps), newExportCmd(deps), newConnectorCmd(deps), newSkillCmd(deps),
	)
	return root
}

// exactArgs wraps cobra.ExactArgs(n) so a wrong argument count is a usageError, classified as
// ExitUserError, instead of the ExitDataError every other non-domain error defaults to.
func exactArgs(n int) cobra.PositionalArgs {
	validate := cobra.ExactArgs(n)
	return func(cmd *cobra.Command, args []string) error {
		return newUsageError(validate(cmd, args))
	}
}

// noArgs wraps cobra.NoArgs the same way exactArgs wraps cobra.ExactArgs.
func noArgs() cobra.PositionalArgs {
	validate := cobra.NoArgs
	return func(cmd *cobra.Command, args []string) error {
		return newUsageError(validate(cmd, args))
	}
}

// registerPersistentFlags defines the flags common to every command, per the technical spec.
func registerPersistentFlags(root *cobra.Command) {
	root.PersistentFlags().Bool("json", false, "output as JSON (snake_case, amounts in minor units)")
	root.PersistentFlags().Bool("csv", false, "output as RFC 4180 CSV")
	root.PersistentFlags().Bool("no-input", false, "never prompt; fail with missing_field instead")
	root.PersistentFlags().Bool("dry-run", false, "show the plan of a write without applying it")
	root.PersistentFlags().Bool("yes", false, "confirm a bulk write or an agent-mode write")
	root.PersistentFlags().String("db", "", "path to the archive (default: XDG data dir)")
	root.PersistentFlags().BoolP("verbose", "v", false, "verbose logging, including connector stderr")
}

// Execute runs the command tree built from deps over args and returns the process exit code. On
// failure it writes the error to deps.Stderr in the format the failing invocation requested.
func Execute(ctx context.Context, deps *Deps, args []string) int {
	root := NewRoot(deps)
	root.SetArgs(args)
	root.SetIn(deps.Stdin)
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	ran, err := root.ExecuteContextC(ctx)
	if err == nil {
		return ExitOK
	}
	rt, rtErr := runtimeOf(ran, deps)
	asJSON := rtErr == nil && rt.format == render.JSON
	if werr := render.WriteError(deps.Stderr, asJSON, err); werr != nil {
		fmt.Fprintln(deps.Stderr, err) //nolint:errcheck // last-resort fallback, stderr itself is failing
	}
	return ExitCode(err)
}
