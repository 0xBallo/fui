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
		return "", nil, false, newUsageError(fmt.Errorf("parse flags: %w", err))
	}
	return first, cmd.Flags().Args(), false, nil
}

// splitOptionalLeadingPositional works like splitLeadingPositional, but the leading positional is
// optional: fui transfer accepts its amount either as "fui transfer 500 --from ... --to ..." or
// as "--amount 500", so args[0] is only consumed as the amount when it looks like one (an
// optional sign followed by a digit); anything else, including every flag, is left to normal
// flag parsing. hasAmount tells the caller whether a positional amount was actually given.
func splitOptionalLeadingPositional(cmd *cobra.Command, args []string) (amount string, hasAmount bool, rest []string, help bool, err error) {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		return "", false, nil, true, nil
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
		if len(args) > 0 {
			amount, hasAmount, args = args[0], true, args[1:]
		}
	} else if len(args) > 0 && looksLikeAmount(args[0]) {
		amount, hasAmount, args = args[0], true, args[1:]
	}
	if err := cmd.Flags().Parse(args); err != nil {
		return "", false, nil, false, newUsageError(fmt.Errorf("parse flags: %w", err))
	}
	return amount, hasAmount, cmd.Flags().Args(), false, nil
}

// looksLikeAmount reports whether s could be the leading positional amount rather than a flag:
// an optional "+"/"-" followed by a digit. This is what distinguishes "500" or "-12,50" (an
// amount) from "--from" or "-c" (a flag) once flag parsing is disabled.
func looksLikeAmount(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	return i < len(s) && s[i] >= '0' && s[i] <= '9'
}
