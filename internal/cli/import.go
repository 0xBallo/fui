package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/module"
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
	cmd.Flags().Bool("accept-transfers", false, "link every proposed transfer pair")
	cmd.Flags().StringSlice("enable-modules", nil, "enable these modules (or \"all\" detected ones) to import their records")
	cmd.AddCommand(newImportUndoCmd(deps), newImportListCmd(deps), newImportDetectCmd(deps))
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
	req, err := importPlanRequest(cmd, deps, file)
	if err != nil {
		return err
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
	opts, err := importApplyOptions(cmd, deps)
	if err != nil {
		return err
	}
	preview := func() error { return render.WriteImportPlan(deps.Stdout, rt.format, plan) }
	return writeChangePreview(cmd, deps, rt, plan.Plan, preview, func() error {
		if plan, opts, err = chooseImportExtras(deps, rt, plan, opts); err != nil {
			return err
		}
		res, err := svc.ApplyImportWith(ctx, plan, opts)
		if err != nil {
			return err
		}
		return render.WriteImportResult(deps.Stdout, rt.format, res)
	})
}

// importPlanRequest builds the plan request from the flags and the enabled modules.
func importPlanRequest(cmd *cobra.Command, deps *Deps, file string) (importing.PlanRequest, error) {
	connector, err := importConnector(cmd, file)
	if err != nil {
		return importing.PlanRequest{}, err
	}
	mappings, err := parseMappings(cmd)
	if err != nil {
		return importing.PlanRequest{}, err
	}
	createAccounts, _ := cmd.Flags().GetBool("create-accounts")
	req := importing.PlanRequest{
		Connector: connector, Fetch: fetchRequest(cmd, file), CreateAccounts: createAccounts, Mappings: mappings,
	}
	req.Modules, err = enabledImportModules(deps)
	return req, err
}

// enabledImportModules returns the enabled modules the plan needs to gate module records and
// installment matching; the zero set when there is no config.
func enabledImportModules(deps *Deps) (module.Set, error) {
	if deps.Config == nil {
		return module.Set{}, nil
	}
	set, err := deps.Config.ModulesEnabled()
	if err != nil {
		return module.Set{}, fmt.Errorf("resolve modules.enabled: %w", err)
	}
	return set, nil
}

// importConnector returns --connector, defaulting to fui for .fui.jsonl files.
func importConnector(cmd *cobra.Command, file string) (string, error) {
	connector, _ := cmd.Flags().GetString("connector")
	if connector != "" {
		return connector, nil
	}
	if !strings.HasSuffix(file, ".fui.jsonl") {
		return "", missingFieldError("connector")
	}
	return "fui", nil
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

// importApplyOptions builds the non-interactive choices from --accept-transfers and
// --enable-modules, and the module set and writer the apply needs to enable modules.
func importApplyOptions(cmd *cobra.Command, deps *Deps) (importing.ApplyOptions, error) {
	accept, _ := cmd.Flags().GetBool("accept-transfers")
	names, _ := cmd.Flags().GetStringSlice("enable-modules")
	opts := importing.ApplyOptions{AcceptTransfers: accept}
	for _, n := range names {
		opts.EnableModules = append(opts.EnableModules, module.ID(strings.TrimSpace(n)))
	}
	if deps.Config != nil {
		set, err := deps.Config.ModulesEnabled()
		if err != nil {
			return opts, fmt.Errorf("resolve modules.enabled: %w", err)
		}
		opts.Modules, opts.ModuleWriter = set, deps.Config
	}
	return opts, nil
}

// chooseImportExtras settles the optional parts of the plan just before it is applied: the
// "all" keyword of --enable-modules, and on a terminal (outside --json, --no-input and agent
// mode) one question per transfer proposal and per disabled module that the flags did not settle.
// Everything left undecided is skipped; skipped transfers are reported on stderr, skipped
// module records by the apply itself.
func chooseImportExtras(
	deps *Deps, rt runtimeFlags, plan importing.ImportPlan, opts importing.ApplyOptions,
) (importing.ImportPlan, importing.ApplyOptions, error) {
	if slices.Contains(opts.EnableModules, "all") {
		opts.EnableModules = slices.DeleteFunc(opts.EnableModules, func(m module.ID) bool { return m == "all" })
		for _, m := range plan.ModuleRecords {
			if !slices.Contains(opts.EnableModules, m.Module) {
				opts.EnableModules = append(opts.EnableModules, m.Module)
			}
		}
	}
	interactive := deps.IsTerminal && !rt.noInput && !rt.agent && rt.format != render.JSON
	if interactive {
		var err error
		if plan, opts, err = askImportChoices(deps, plan, opts); err != nil {
			return plan, opts, err
		}
	}
	if left := pendingTransfers(plan, opts); left > 0 {
		if _, err := fmt.Fprintf(deps.Stderr, "warning: %d transfer proposal(s) skipped; use --accept-transfers to link them\n", left); err != nil {
			return plan, opts, fmt.Errorf("write transfer warning: %w", err)
		}
	}
	return plan, opts, nil
}

func pendingTransfers(plan importing.ImportPlan, opts importing.ApplyOptions) int {
	if opts.AcceptTransfers {
		return 0
	}
	n := 0
	for _, t := range plan.Transfers {
		if !t.Accept {
			n++
		}
	}
	return n
}

// askImportChoices asks, in one form (the accessible prompt buffers its input, so several forms
// would lose answers), whether to accept each transfer proposal and to enable or skip each
// disabled module.
func askImportChoices(deps *Deps, plan importing.ImportPlan, opts importing.ApplyOptions) (importing.ImportPlan, importing.ApplyOptions, error) {
	var fields []huh.Field
	accept := make([]bool, len(plan.Transfers))
	if !opts.AcceptTransfers {
		for i, t := range plan.Transfers {
			accept[i] = t.Accept
			fields = append(fields, huh.NewConfirm().
				Title("Link as a transfer? "+render.TransferProposalText(i, t)).Value(&accept[i]))
		}
	}
	var asked []importing.ModuleRecords
	for _, m := range plan.ModuleRecords {
		if !slices.Contains(opts.EnableModules, m.Module) {
			asked = append(asked, m)
		}
	}
	choices := make([]string, len(asked))
	for i, m := range asked {
		choices[i] = "skip"
		fields = append(fields, huh.NewSelect[string]().
			Title(fmt.Sprintf("%d record(s) of the disabled module %s", m.Count, m.Module)).
			Options(huh.NewOption("Skip them", "skip"), huh.NewOption("Enable "+string(m.Module), "enable")).
			Value(&choices[i]))
	}
	if len(fields) == 0 {
		return plan, opts, nil
	}
	form := huh.NewForm(huh.NewGroup(fields...)).WithAccessible(true).WithInput(byteReader{deps.Stdin}).WithOutput(deps.Stdout)
	if err := form.Run(); err != nil {
		return plan, opts, fmt.Errorf("prompt import choices: %w", err)
	}
	if !opts.AcceptTransfers {
		for i := range plan.Transfers {
			if err := plan.SetAcceptTransfer(i, accept[i]); err != nil {
				return plan, opts, err
			}
		}
	}
	for i, m := range asked {
		if choices[i] == "enable" {
			opts.EnableModules = append(opts.EnableModules, m.Module)
		}
	}
	return plan, opts, nil
}

// byteReader hands out one byte per Read: the accessible prompt wraps its input in a fresh
// buffered reader for every field, which would swallow the answers of the following fields.
type byteReader struct{ r io.Reader }

func (b byteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return b.r.Read(p[:1]) //nolint:wrapcheck // an io.Reader must return io.EOF unwrapped
}
