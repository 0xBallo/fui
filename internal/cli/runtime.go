package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// runtimeFlags are the resolved persistent flags of one command invocation. They are computed on
// demand from cmd.Flags() rather than cached, so a command that must reparse flags after taking
// over raw argv (fui add's leading amount) sees the same values as every other command.
type runtimeFlags struct {
	format  render.Format
	noInput bool
	dryRun  bool
	yes     bool
	verbose bool
	agent   bool
	color   bool
}

// runtimeOf resolves the persistent flags of cmd. FUI_AGENT (Config.AgentMode) forces --no-input
// and --json, per the agent-mode requirement of the technical spec.
func runtimeOf(cmd *cobra.Command, deps *Deps) (runtimeFlags, error) {
	isJSON, _ := cmd.Flags().GetBool("json")
	isCSV, _ := cmd.Flags().GetBool("csv")
	if isJSON && isCSV {
		return runtimeFlags{}, newUsageError(errors.New("--json and --csv are mutually exclusive"))
	}
	noInput, _ := cmd.Flags().GetBool("no-input")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	verbose, _ := cmd.Flags().GetBool("verbose")
	agent := deps.Config != nil && deps.Config.AgentMode
	if agent {
		isJSON, noInput = true, true
	}
	format := render.Table
	switch {
	case isJSON:
		format = render.JSON
	case isCSV:
		format = render.CSV
	}
	return runtimeFlags{
		format: format, noInput: noInput, dryRun: dryRun, yes: yes, verbose: verbose,
		agent: agent, color: deps.IsTerminal && os.Getenv("NO_COLOR") == "",
	}, nil
}
