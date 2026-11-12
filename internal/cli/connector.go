package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newConnectorCmd groups the connector discovery and diagnostic subcommands.
func newConnectorCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "connector", Short: "List, show or test import connectors"}
	cmd.AddCommand(newConnectorListCmd(deps), newConnectorShowCmd(deps), newConnectorTestCmd(deps))
	return cmd
}

func newConnectorListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List the discovered connectors",
		Example: "  fui connector list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireImporting(deps)
			if err != nil {
				return err
			}
			cs, err := svc.ListConnectors(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteConnectorList(deps.Stdout, rt.format, cs)
		},
	}
}

func newConnectorShowCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "show NAME",
		Short:   "Show the metadata of one connector",
		Args:    exactArgs(1),
		Example: "  fui connector show csv",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			svc, err := requireImporting(deps)
			if err != nil {
				return err
			}
			d, err := svc.ShowConnector(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return render.WriteConnectorDetail(deps.Stdout, rt.format, d)
		},
	}
}

func newConnectorTestCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "test NAME FILE",
		Short:   "Fetch and validate a connector stream without importing it",
		Args:    exactArgs(2),
		Example: "  fui connector test csv statement.csv --profile mybank",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectorTest(cmd, deps, args[0], args[1])
		},
	}
	cmd.Flags().String("profile", "", "csv profile name (import.profiles.<name>)")
	cmd.Flags().String("account", "", "connector account ref or name (csv connector)")
	cmd.Flags().String("since", "", "only fetch records on or after this date")
	return cmd
}

func runConnectorTest(cmd *cobra.Command, deps *Deps, name, file string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireImporting(deps)
	if err != nil {
		return err
	}
	rep, err := svc.TestConnector(cmd.Context(), name, fetchRequest(cmd, file))
	if err != nil {
		if rt.verbose && rep.Stderr != "" {
			fmt.Fprintln(deps.Stderr, rep.Stderr) //nolint:errcheck // best effort alongside the error already returned
		}
		return err
	}
	return render.WriteTestReport(deps.Stdout, rt.format, rt.verbose, rep)
}
