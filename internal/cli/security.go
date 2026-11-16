package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/module"
)

// NewSecurityCmd builds the security command group.
func NewSecurityCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Add, list and price securities",
	}
	cmd.AddCommand(
		newSecurityAddCmd(deps, svc),
		newSecurityListCmd(deps, svc),
		newSecurityPriceCmd(deps, svc),
	)
	annotateModule(cmd, module.Investments)
	return cmd
}

func newSecurityAddCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "Add a security",
		Args:  exactArgs(1),
		Example: "  fui security add \"VWCE ETF\" --kind etf --ticker VWCE --currency EUR\n" +
			"  fui security add Bitcoin --kind crypto --ticker BTC --currency EUR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecurityAdd(cmd, deps, svc, args[0])
		},
	}
	cmd.Flags().String("kind", "", "security kind: stock, etf, bond, fund or crypto")
	cmd.Flags().String("ticker", "", "ticker (required when --isin is missing)")
	cmd.Flags().String("isin", "", "ISIN code (required when --ticker is missing)")
	cmd.Flags().String("currency", "", "ISO 4217 currency")
	cmd.Flags().Int("quantity-scale", 0, "quantity decimals (default: 8 for crypto, 0 for others)")
	return cmd
}

func runSecurityAdd(cmd *cobra.Command, deps *Deps, svc *investing.Service, name string) error {
	if err := requireInvesting(svc); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	kind, _ := cmd.Flags().GetString("kind")
	ticker, _ := cmd.Flags().GetString("ticker")
	isin, _ := cmd.Flags().GetString("isin")
	currency, _ := cmd.Flags().GetString("currency")
	var scale *int
	if cmd.Flags().Changed("quantity-scale") {
		raw, _ := cmd.Flags().GetInt("quantity-scale")
		scale = &raw
	}
	ctx := cmd.Context()
	plan, err := svc.PlanAddSecurity(ctx, investing.AddSecurityRequest{
		Name: name, Kind: kind, Ticker: ticker, ISIN: isin, Currency: currency, QuantityScale: scale,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		sec, err := svc.ApplyAddSecurity(ctx, plan)
		if err != nil {
			return err
		}
		return render.WriteSecurity(deps.Stdout, rt.format, sec)
	})
}

func newSecurityListCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List securities",
		Args:    noArgs(),
		Example: "  fui security list --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireInvesting(svc); err != nil {
				return err
			}
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			secs, err := svc.ListSecurities(cmd.Context())
			if err != nil {
				return err
			}
			return render.WriteSecurityList(deps.Stdout, rt.format, secs)
		},
	}
}

func newSecurityPriceCmd(deps *Deps, svc *investing.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "price SECURITY PRICE",
		Short:   "Set a manual price for a security",
		Args:    exactArgs(2),
		Example: "  fui security price VWCE 127.35 --date 2026-03-01\n  fui security price BTC 95000 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecurityPrice(cmd, deps, svc, args[0], args[1])
		},
	}
	cmd.Flags().String("date", "", "price date (default: today)")
	return cmd
}

func runSecurityPrice(cmd *cobra.Command, deps *Deps, svc *investing.Service, ref, price string) error {
	if err := requireInvesting(svc); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	date, _ := cmd.Flags().GetString("date")
	ctx := cmd.Context()
	plan, err := svc.PlanSetPrice(ctx, investing.SetPriceRequest{Security: ref, Date: date, Price: price})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := svc.ApplySetPrice(ctx, plan); err != nil {
			return err
		}
		return render.WriteSecurityPrice(deps.Stdout, rt.format, deps.Locale(), render.SecurityPriceView{
			SecurityID: int64(plan.Price.Security),
			Date:       plan.Price.Date,
			Price:      plan.Price.Price,
		})
	})
}

func requireInvesting(svc *investing.Service) error {
	if svc == nil {
		return fmt.Errorf("fui: investing service not configured")
	}
	return nil
}
