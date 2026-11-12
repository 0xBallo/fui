package cli

import (
	"context"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// newTxTagCmd adds one or more tags to a transaction, keeping any it already carries.
func newTxTagCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "tag ID TAG...",
		Short:   "Add tags to a transaction",
		Args:    minArgs(2),
		Example: "  fui tx tag 42 vacanze-2026",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxTag(cmd, deps, args[0], args[1:])
		},
	}
}

// newTxUntagCmd removes one or more tags from a transaction.
func newTxUntagCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "untag ID TAG...",
		Short:   "Remove tags from a transaction",
		Args:    minArgs(2),
		Example: "  fui tx untag 42 vacanze-2026",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTxUntag(cmd, deps, args[0], args[1:])
		},
	}
}

func runTxTag(cmd *cobra.Command, deps *Deps, idArg string, tags []string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	id, err := parseTxID(idArg)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanTagTransaction(ctx, app.TagTransactionRequest{ID: id, Add: tags})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Service.ApplyTagTransaction(ctx, plan); err != nil {
			return err
		}
		return renderTxTags(ctx, deps, rt, id, plan.Tags)
	})
}

func runTxUntag(cmd *cobra.Command, deps *Deps, idArg string, tags []string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	id, err := parseTxID(idArg)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanUntagTransaction(ctx, app.UntagTransactionRequest{ID: id, Remove: tags})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Service.ApplyUntagTransaction(ctx, plan); err != nil {
			return err
		}
		return renderTxTags(ctx, deps, rt, id, plan.Tags)
	})
}

// parseTxID parses idArg as a transaction id, failing with missing_field like the existing tx
// commands (tx show, tx edit) do on a malformed id.
func parseTxID(idArg string) (domain.TransactionID, error) {
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil {
		return 0, missingFieldError("id")
	}
	return domain.TransactionID(id), nil
}

// renderTxTags resolves ids to tag names and renders the transaction's resulting tag set.
func renderTxTags(ctx context.Context, deps *Deps, rt runtimeFlags, id domain.TransactionID, ids []domain.TagID) error {
	tags, err := deps.Service.ListTags(ctx)
	if err != nil {
		return err
	}
	names := make(map[domain.TagID]string, len(tags))
	for _, tg := range tags {
		names[tg.ID] = tg.Name
	}
	out := make([]string, len(ids))
	for i, tid := range ids {
		out[i] = names[tid]
	}
	sort.Strings(out)
	return render.WriteTxTags(deps.Stdout, rt.format, int64(id), out)
}
