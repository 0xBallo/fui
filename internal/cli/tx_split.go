package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newTxSplitCmd divides a transaction into categorized rows that must sum to its amount.
func newTxSplitCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     `split ID --row "cat=amount[:note]"...`,
		Short:   "Split a transaction across categories",
		Args:    exactArgs(1),
		Example: `  fui tx split 42 --row "Groceries=-30,00" --row "Household=-10,00:cleaning"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxSplit(cmd, deps, args[0])
		},
	}
	cmd.Flags().StringArray("row", nil, `split row as "category=amount[:note]" (repeatable, at least 2 required)`)
	return cmd
}

// newTxUnsplitCmd removes the split rows of a transaction, leaving it with a single category.
func newTxUnsplitCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "unsplit ID",
		Short:   "Remove the split rows of a transaction",
		Args:    exactArgs(1),
		Example: "  fui tx unsplit 42",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxUnsplit(cmd, deps, args[0])
		},
	}
}

func runTxSplit(cmd *cobra.Command, deps *Deps, idArg string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	id, err := parseTxID(idArg)
	if err != nil {
		return err
	}
	raw, _ := cmd.Flags().GetStringArray("row")
	rows := make([]app.SplitRowRequest, len(raw))
	for i, r := range raw {
		row, err := parseSplitRow(r)
		if err != nil {
			return err
		}
		rows[i] = row
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanSplitTx(ctx, app.SplitTxRequest{ID: id, Rows: rows})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Service.ApplySplitTx(ctx, plan); err != nil {
			return err
		}
		return renderSplitsByID(ctx, deps, rt, id)
	})
}

func runTxUnsplit(cmd *cobra.Command, deps *Deps, idArg string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	id, err := parseTxID(idArg)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanUnsplitTx(ctx, app.UnsplitTxRequest{ID: id})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Service.ApplyUnsplitTx(ctx, plan); err != nil {
			return err
		}
		return renderTxByID(ctx, deps, rt, id)
	})
}

// parseSplitRow parses one --row value "category=amount[:note]" into a SplitRowRequest. The
// amount is split off at the first ":", since neither a category name nor a parsed amount uses
// that character.
func parseSplitRow(raw string) (app.SplitRowRequest, error) {
	eq := strings.Index(raw, "=")
	if eq <= 0 || eq == len(raw)-1 {
		return app.SplitRowRequest{}, newUsageError(fmt.Errorf(`invalid --row %q, want "category=amount[:note]"`, raw))
	}
	category, rest := raw[:eq], raw[eq+1:]
	amount, note := rest, ""
	if i := strings.Index(rest, ":"); i >= 0 {
		amount, note = rest[:i], rest[i+1:]
	}
	if amount == "" {
		return app.SplitRowRequest{}, newUsageError(fmt.Errorf(`invalid --row %q, want "category=amount[:note]"`, raw))
	}
	return app.SplitRowRequest{Category: category, Amount: amount, Note: note}, nil
}

// renderSplitsByID fetches and renders the split rows of id, resolving each row's category to its
// name.
func renderSplitsByID(ctx context.Context, deps *Deps, rt runtimeFlags, id domain.TransactionID) error {
	rows, err := deps.Service.ListSplits(ctx, id)
	if err != nil {
		return err
	}
	cats, err := deps.Service.ListCategories(ctx)
	if err != nil {
		return err
	}
	names := make(map[domain.CategoryID]string, len(cats))
	for _, c := range cats {
		names[c.ID] = c.Name
	}
	views := make([]render.SplitRowView, len(rows))
	for i, r := range rows {
		views[i] = render.SplitRowView{Category: names[r.Category], Amount: r.Amount, Note: r.Note}
	}
	return render.WriteSplitList(deps.Stdout, rt.format, deps.Locale(), rt.color, views)
}
