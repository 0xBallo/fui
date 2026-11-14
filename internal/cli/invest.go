package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// NewInvestCmd builds the invest command group.
func NewInvestCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invest",
		Short: "Record buy, sell and dividend operations",
	}
	cmd.AddCommand(
		newInvestBuyCmd(deps, svc),
		newInvestSellCmd(deps, svc),
		newInvestDividendCmd(deps, svc),
	)
	annotateModule(cmd, module.Investments)
	return cmd
}

func newInvestBuyCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buy SECURITY",
		Short: "Record a buy operation",
		Args:  exactArgs(1),
		Example: "  fui invest buy VWCE --account Broker --quantity 10 --price 100 --fees 1\n" +
			"  fui invest buy BTC --account Exchange --quantity 0.01 --price 95000 --cash-account Cash EUR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvestOp(cmd, deps, svc, domain.OpBuy, args[0])
		},
	}
	registerInvestOpFlags(cmd)
	return cmd
}

func newInvestSellCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sell SECURITY",
		Short:   "Record a sell operation",
		Args:    exactArgs(1),
		Example: "  fui invest sell VWCE --account Broker --quantity 2 --price 110 --fees 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvestOp(cmd, deps, svc, domain.OpSell, args[0])
		},
	}
	registerInvestOpFlags(cmd)
	return cmd
}

func newInvestDividendCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dividend SECURITY",
		Short:   "Record a dividend operation",
		Args:    exactArgs(1),
		Example: "  fui invest dividend VWCE --account Broker --quantity 8 --price 0.5 --cash-account Cash",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvestOp(cmd, deps, svc, domain.OpDividend, args[0])
		},
	}
	registerInvestOpFlags(cmd)
	return cmd
}

func registerInvestOpFlags(cmd *cobra.Command) {
	cmd.Flags().String("account", "", "investment account holding the position")
	cmd.Flags().String("cash-account", "", "optional linked cash account")
	cmd.Flags().String("date", "", "operation date (default: today)")
	cmd.Flags().String("quantity", "", "quantity of units")
	cmd.Flags().String("price", "", "unit price in the security currency")
	cmd.Flags().String("fees", "", "fees in the security currency (default: 0)")
}

func runInvestOp(cmd *cobra.Command, deps *Deps, svc *investing.Service, kind domain.SecurityOpKind, security string) error {
	if err := requireInvesting(svc); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	account, _ := cmd.Flags().GetString("account")
	cashAccount, _ := cmd.Flags().GetString("cash-account")
	date, _ := cmd.Flags().GetString("date")
	quantity, _ := cmd.Flags().GetString("quantity")
	price, _ := cmd.Flags().GetString("price")
	fees, _ := cmd.Flags().GetString("fees")
	req := investing.OpRequest{
		Kind: kind, Account: account, CashAccount: cashAccount, Security: security,
		Date: date, Quantity: quantity, Price: price, Fees: fees,
	}
	ctx := cmd.Context()
	var plan investing.OpPlan
	switch kind {
	case domain.OpBuy:
		plan, err = svc.PlanBuy(ctx, req)
	case domain.OpSell:
		plan, err = svc.PlanSell(ctx, req)
	case domain.OpDividend:
		plan, err = svc.PlanDividend(ctx, req)
	case domain.OpCoupon:
		fallthrough
	default:
		return &domain.Error{Code: domain.CodeInvalidValue, Message: "unsupported operation kind"}
	}
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		op, err := svc.ApplyOp(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteSecurityOp(deps.Stdout, rt.format, deps.Locale(), op)
	})
}
