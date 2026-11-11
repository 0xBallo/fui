package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// splitLeadingPositional works around a pflag quirk: a bare positional argument that looks like
// a negative number (e.g. "-12,50") is parsed as an unknown shorthand flag and rejected. The
// commands that take a signed amount as their first positional argument (fui add) call this
// instead of relying on cobra's normal flag parsing, disabled on their Command.
//
// It takes args[0] (or args[1] after a literal "--" separator, so "fui add -- -12,50 ..." also
// works) as first, reparses cmd.Flags() over the remaining tokens so every local and inherited
// flag works normally, and returns the positional arguments left after that. help is true when
// the caller asked for -h/--help, which DisableFlagParsing stops cobra from handling on its own.
func splitLeadingPositional(cmd *cobra.Command, args []string) (first string, rest []string, help bool, err error) {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		return "", nil, true, nil
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return "", nil, false, missingFieldError("amount")
	}
	first, tail := args[0], args[1:]
	if err := cmd.Flags().Parse(tail); err != nil {
		return "", nil, false, fmt.Errorf("parse flags: %w", err)
	}
	return first, cmd.Flags().Args(), false, nil
}
