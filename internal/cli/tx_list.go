package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newTxListCmd lists transactions, newest first by default, with every filterflags.go flag plus
// --after-date/--after-id paging.
func newTxListCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List transactions",
		Example: "  fui tx list --account Checking --from 2026-01-01 --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTxList(cmd, deps)
		},
	}
	registerFilterFlags(cmd)
	cmd.Flags().String("after-date", "", "resume after this ISO date (from the previous page)")
	cmd.Flags().Int64("after-id", 0, "resume after this transaction id (from the previous page)")
	return cmd
}

func runTxList(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := txListRequest(cmd)
	if err != nil {
		return err
	}
	page, err := deps.Service.ListTx(cmd.Context(), req)
	if err != nil {
		return err
	}
	if err := render.WriteTxPage(deps.Stdout, rt.format, deps.Locale(), rt.color, page); err != nil {
		return err
	}
	if page.Next != nil && rt.format == render.Table {
		if _, err := fmt.Fprintf(deps.Stdout, "More rows: --after-date %s --after-id %d\n", page.Next.Date, page.Next.ID); err != nil {
			return fmt.Errorf("write pagination hint: %w", err)
		}
	}
	return nil
}

// txListRequest reads the shared filter flags and tx list's own paging flags into a
// ListTxRequest.
func txListRequest(cmd *cobra.Command) (app.ListTxRequest, error) {
	req, err := readFilterFlags(cmd)
	if err != nil {
		return app.ListTxRequest{}, err
	}
	afterDate, _ := cmd.Flags().GetString("after-date")
	if afterDate != "" {
		d, err := domain.ParseDate(afterDate)
		if err != nil {
			return app.ListTxRequest{}, err
		}
		id, _ := cmd.Flags().GetInt64("after-id")
		req.After = &app.TxCursor{Date: d, ID: domain.TransactionID(id)}
	}
	return req, nil
}
