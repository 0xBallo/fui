package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

func newSecurityEditCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit REF",
		Short: "Edit the name, ticker or ISIN of a security",
		Args:  exactArgs(1),
		Example: "  fui security edit VWCE --name \"Vanguard FTSE All-World\"\n" +
			"  fui security edit BTC --ticker XBT --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecurityEdit(cmd, deps, svc, args[0])
		},
	}
	cmd.Flags().String("name", "", "new name")
	cmd.Flags().String("ticker", "", "new ticker")
	cmd.Flags().String("isin", "", "new ISIN")
	return cmd
}

func runSecurityEdit(cmd *cobra.Command, deps *Deps, svc *investing.Service, ref string) error {
	if err := requireInvesting(svc); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req := investing.EditSecurityRequest{Ref: ref}
	for flag, dst := range map[string]**string{"name": &req.Name, "ticker": &req.Ticker, "isin": &req.ISIN} {
		if cmd.Flags().Changed(flag) {
			v, _ := cmd.Flags().GetString(flag)
			*dst = &v
		}
	}
	if req.Name == nil && req.Ticker == nil && req.ISIN == nil {
		return &domain.Error{Code: domain.ErrMissingField.Code, Message: "nothing to change: pass --name, --ticker or --isin"}
	}
	ctx := cmd.Context()
	plan, err := svc.PlanEditSecurity(ctx, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		sec, err := svc.ApplyEditSecurity(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteSecurity(deps.Stdout, rt.format, sec)
	})
}

func newSecurityRemoveCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "rm REF",
		Short: "Remove a security and its manual prices (refused while it has operations)",
		Args:  exactArgs(1),
		Example: "  fui security rm OLDCO --dry-run\n" +
			"  fui security rm OLDCO --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInvesting(svc); err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanRemoveSecurity(ctx, args[0])
			if err != nil {
				return err
			}
			plan.Bulk = true
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyRemoveSecurity(ctx, plan); err != nil {
					return err
				}
				return writeRemoved(deps, rt, int64(plan.Security.ID), fmt.Sprintf("Removed security %q.", plan.Security.Name))
			})
		},
	}
}

// writeRemoved reports a deletion: {"removed": id} for --json, note otherwise.
func writeRemoved(deps *Deps, rt runtimeFlags, id int64, note string) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, struct {
			Removed int64 `json:"removed"`
		}{Removed: id})
	}
	if _, err := fmt.Fprintln(deps.Stdout, note); err != nil {
		return fmt.Errorf("write removal note: %w", err)
	}
	return nil
}
