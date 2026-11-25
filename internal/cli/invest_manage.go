package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

func newInvestCouponCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "coupon SECURITY",
		Short:   "Record a bond coupon operation",
		Args:    exactArgs(1),
		Example: "  fui invest coupon BTP2030 --account Broker --quantity 10 --price 1.5 --cash-account Cash",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvestOp(cmd, deps, svc, domain.OpCoupon, args[0])
		},
	}
	registerInvestOpFlags(cmd)
	return cmd
}

func newInvestListCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List investment operations",
		Args:  noArgs(),
		Example: "  fui invest list --account Broker --kind buy\n" +
			"  fui invest list --security VWCE --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireInvesting(svc); err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			account, _ := cmd.Flags().GetString("account")
			security, _ := cmd.Flags().GetString("security")
			kind, _ := cmd.Flags().GetString("kind")
			lines, err := svc.ListOps(cmd.Context(), investing.ListOpsRequest{Account: account, Security: security, Kind: kind})
			if err != nil {
				return err
			}
			return render.WriteOpList(deps.Stdout, rt.format, deps.Locale(), rt.color, lines)
		},
	}
	cmd.Flags().String("account", "", "account name filter")
	cmd.Flags().String("security", "", "security filter (ticker, ISIN or name)")
	cmd.Flags().String("kind", "", "operation kind filter: buy, sell, dividend or coupon")
	return cmd
}

func parseOpID(arg string) (domain.SecurityOpID, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || id <= 0 {
		return 0, &domain.Error{
			Code: domain.ErrInvalidValue.Code, Message: fmt.Sprintf("invalid operation id %q", arg),
			Details: map[string]any{"field": "id", "value": arg},
		}
	}
	return domain.SecurityOpID(id), nil
}

func newInvestEditCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit the date, quantity, price or fees of an operation",
		Args:  exactArgs(1),
		Example: "  fui invest edit 3 --quantity 12 --price 101.5\n" +
			"  fui invest edit 3 --date 2026-03-02 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvestEdit(cmd, deps, svc, args[0])
		},
	}
	cmd.Flags().String("date", "", "new operation date")
	cmd.Flags().String("quantity", "", "new quantity of units")
	cmd.Flags().String("price", "", "new unit price")
	cmd.Flags().String("fees", "", "new fees (empty for zero)")
	return cmd
}

func runInvestEdit(cmd *cobra.Command, deps *Deps, svc *investing.Service, idArg string) error {
	if err := requireInvesting(svc); err != nil {
		return err
	}
	id, err := parseOpID(idArg)
	if err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req := investing.EditOpRequest{ID: id}
	for flag, dst := range map[string]**string{"date": &req.Date, "quantity": &req.Quantity, "price": &req.Price, "fees": &req.Fees} {
		if cmd.Flags().Changed(flag) {
			v, _ := cmd.Flags().GetString(flag)
			*dst = &v
		}
	}
	if req.Date == nil && req.Quantity == nil && req.Price == nil && req.Fees == nil {
		return &domain.Error{Code: domain.ErrMissingField.Code, Message: "nothing to change: pass --date, --quantity, --price or --fees"}
	}
	ctx := cmd.Context()
	plan, err := svc.PlanEditOp(ctx, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		op, err := svc.ApplyEditOp(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteSecurityOp(deps.Stdout, rt.format, deps.Locale(), op)
	})
}

func newInvestRemoveCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "rm ID",
		Short: "Remove an operation together with its linked cash transaction",
		Long: "Remove an investment operation. When the operation has a linked cash transaction, that\n" +
			"transaction is deleted too; both deletions are listed in the --dry-run output.",
		Args: exactArgs(1),
		Example: "  fui invest rm 3 --dry-run\n" +
			"  fui invest rm 3 --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInvesting(svc); err != nil {
				return err
			}
			id, err := parseOpID(args[0])
			if err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanRemoveOp(ctx, id)
			if err != nil {
				return err
			}
			plan.Bulk = true
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyRemoveOp(ctx, plan); err != nil {
					return err
				}
				note := fmt.Sprintf("Removed operation %d.", id)
				if plan.Op.CashTx != 0 {
					note = fmt.Sprintf("Removed operation %d and its cash transaction %d.", id, plan.Op.CashTx)
				}
				return writeRemoved(deps, rt, int64(id), note)
			})
		},
	}
}
