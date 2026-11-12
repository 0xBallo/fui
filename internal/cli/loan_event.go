package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/cli/render"
)

// newLoanEventCmd records a rate change, renegotiation or prepayment, and computes the new
// schedule version it produces from its date on.
func newLoanEventCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event NAME",
		Short: "Record a loan rate change, renegotiation or prepayment",
		Args:  exactArgs(1),
		Example: "  fui loan event Mutuo --kind rate_change --date 2026-01-01 --rate 2,80\n" +
			"  fui loan event Mutuo --kind prepayment --date 2026-01-01 --prepaid 5000 --mode reduce_term",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoanEvent(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("kind", "", "rate_change, renegotiation or prepayment (required)")
	cmd.Flags().String("date", "", "the date the event takes effect (required)")
	cmd.Flags().String("rate", "", "new annual nominal rate as a percentage (rate_change, renegotiation)")
	cmd.Flags().Int("term-months", 0, "new remaining term in months (renegotiation, optional)")
	cmd.Flags().String("prepaid", "", "amount prepaid (prepayment)")
	cmd.Flags().String("mode", "", "reduce_installment or reduce_term (prepayment)")
	return cmd
}

func runLoanEvent(cmd *cobra.Command, deps *Deps, ref string) error {
	if err := requireLoans(deps); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	enabled, err := enabledModulesFromDeps(deps)
	if err != nil {
		return err
	}
	kind, _ := cmd.Flags().GetString("kind")
	date, _ := cmd.Flags().GetString("date")
	rate, _ := cmd.Flags().GetString("rate")
	prepaid, _ := cmd.Flags().GetString("prepaid")
	mode, _ := cmd.Flags().GetString("mode")
	req := lending.EventRequest{Loan: ref, Kind: kind, Date: date, Rate: rate, Prepaid: prepaid, Mode: mode}
	if cmd.Flags().Changed("term-months") {
		termMonths, _ := cmd.Flags().GetInt("term-months")
		req.TermMonths = &termMonths
	}
	ctx := cmd.Context()
	plan, err := deps.Loans.PlanEvent(ctx, enabled, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		if err := deps.Loans.ApplyEvent(ctx, plan); err != nil {
			return err
		}
		return render.WriteLoanSchedule(deps.Stdout, rt.format, deps.Locale(), plan.Schedule)
	})
}
