package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// newImportCmd imports a connector stream, plus its undo and list subcommands.
func newImportCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import FILE",
		Short: "Import transactions from a connector",
		Args:  exactArgs(1),
		Example: "  fui import statement.csv --connector csv --profile mybank --account Checking\n" +
			"  fui import export.fui.jsonl",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("connector", "", "connector name (default: fui for .fui.jsonl files)")
	cmd.Flags().String("profile", "", "csv profile name (import.profiles.<name>)")
	cmd.Flags().String("account", "", "connector account ref or name (csv connector)")
	cmd.Flags().StringArray("map", nil, "bind a connector ref to an account: ref=account (repeatable)")
	cmd.Flags().Bool("create-accounts", false, "create accounts for refs still unmapped")
	cmd.Flags().String("since", "", "only fetch records on or after this date")
	cmd.AddCommand(newImportUndoCmd(deps), newImportListCmd(deps))
	return cmd
}

func runImport(cmd *cobra.Command, deps *Deps, file string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireImporting(deps)
	if err != nil {
		return err
	}
	connector, _ := cmd.Flags().GetString("connector")
	if connector == "" {
		if !strings.HasSuffix(file, ".fui.jsonl") {
			return missingFieldError("connector")
		}
		connector = "fui"
	}
	mappings, err := parseMappings(cmd)
	if err != nil {
		return err
	}
	createAccounts, _ := cmd.Flags().GetBool("create-accounts")
	req := importing.PlanRequest{
		Connector: connector, Fetch: fetchRequest(cmd, file), CreateAccounts: createAccounts, Mappings: mappings,
	}
	ctx := cmd.Context()
	plan, err := svc.PlanImport(ctx, req)
	if err != nil {
		return err
	}
	if len(plan.Unmapped) > 0 && !rt.dryRun && !rt.agent && !rt.noInput && deps.IsTerminal {
		if plan, err = resolveUnmapped(cmd, deps, svc, req, plan); err != nil {
			return err
		}
	}
	preview := func() error { return render.WriteImportPlan(deps.Stdout, rt.format, plan) }
	return writeChangePreview(cmd, deps, rt, plan.Plan, preview, func() error {
		res, err := svc.ApplyImport(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteImportResult(deps.Stdout, rt.format, res)
	})
}

// fetchRequest builds the connector fetch request shared by `import` and `connector test` from
// their --profile, --account and --since flags.
func fetchRequest(cmd *cobra.Command, file string) fuiproto.FetchRequest {
	config := map[string]any{}
	if profile, _ := cmd.Flags().GetString("profile"); profile != "" {
		config["profile"] = profile
	}
	if account, _ := cmd.Flags().GetString("account"); account != "" {
		config["account"] = account
	}
	fetch := fuiproto.FetchRequest{File: &file, Config: config}
	if since, _ := cmd.Flags().GetString("since"); since != "" {
		fetch.Since = &since
	}
	return fetch
}

// parseMappings splits each --map ref=account flag into the PlanRequest.Mappings map.
func parseMappings(cmd *cobra.Command) (map[string]string, error) {
	args, _ := cmd.Flags().GetStringArray("map")
	if len(args) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(args))
	for _, a := range args {
		ref, acct, ok := strings.Cut(a, "=")
		if !ok || ref == "" || acct == "" {
			return nil, newUsageError(fmt.Errorf("invalid --map %q: want ref=account", a))
		}
		out[ref] = acct
	}
	return out, nil
}
