package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/cli/render"
)

// newExportCmd writes the matching transactions, or (--all) the full archive as one file per
// entity into a directory.
func newExportCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transactions, or the full archive with --all",
		Example: "  fui export --account Checking --format csv --output checking.csv\n" +
			"  fui export --all --output ./dump",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runExport(cmd, deps)
		},
	}
	registerTxListFlags(cmd)
	cmd.Flags().String("format", "csv", "csv, json or fui (fui only without --all)")
	cmd.Flags().Bool("all", false, "dump the full archive into --output, one file per entity")
	cmd.Flags().String("output", "", "output file (export) or directory (--all); stdout when omitted")
	return cmd
}

func runExport(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireImporting(deps)
	if err != nil {
		return err
	}
	format, _ := cmd.Flags().GetString("format")
	all, _ := cmd.Flags().GetBool("all")
	output, _ := cmd.Flags().GetString("output")
	ctx := cmd.Context()
	if all {
		return runDump(ctx, deps, svc, rt, format, output)
	}
	req, err := txListRequest(cmd)
	if err != nil {
		return err
	}
	filter, err := deps.Service.ResolveTxFilter(ctx, req)
	if err != nil {
		return err
	}
	w := deps.Stdout
	if output != "" {
		f, ferr := os.Create(output) // #nosec G304 -- output path is provided by the user command.
		if ferr != nil {
			return fmt.Errorf("create output file: %w", ferr)
		}
		defer f.Close() //nolint:errcheck // the file was only written to, nothing left to flush on error
		w = f
	}
	n, err := svc.Export(ctx, importing.ExportRequest{Filter: filter, Format: importing.ExportFormat(format)}, w)
	if err != nil {
		return err
	}
	if output == "" {
		return nil // stdout already carries the exported stream; no extra output to mix in
	}
	return writeExportConfirmation(deps, rt, n, output)
}

func runDump(ctx context.Context, deps *Deps, svc *importing.Service, rt runtimeFlags, format, output string) error {
	if format == string(importing.FormatFUI) {
		return newUsageError(fmt.Errorf("--all does not support --format fui"))
	}
	if output == "" {
		return missingFieldError("output")
	}
	if err := os.MkdirAll(output, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := svc.Dump(ctx, importing.DumpFormat(format), dumpFile(output)); err != nil {
		return err
	}
	return writeExportConfirmation(deps, rt, 0, output)
}

// dumpFile creates the output file of one dumped entity inside dir.
func dumpFile(dir string) importing.DumpFile {
	return func(name string) (io.WriteCloser, error) {
		f, err := os.Create(filepath.Join(dir, name)) // #nosec G304 -- dir is the output directory the user gave.
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", name, err)
		}
		return f, nil
	}
}

func writeExportConfirmation(deps *Deps, rt runtimeFlags, transactions int, path string) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Transactions int    `json:"transactions,omitempty"`
			Path         string `json:"path"`
		}{transactions, path})
	}
	_, err := fmt.Fprintf(deps.Stdout, "Exported to %s.\n", path)
	if err != nil {
		return fmt.Errorf("write export confirmation: %w", err)
	}
	return nil
}
