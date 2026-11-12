package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/cli/render"
)

// newLoanAddCmd creates a loan or mortgage: its liability account, and either a computed French
// amortization schedule or one read from an imported fui/1 JSONL file of installment records.
func newLoanAddCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "Add a loan or mortgage",
		Args:  exactArgs(1),
		Example: "  fui loan add Mutuo --kind mortgage --principal 200000 --term-months 240 \\\n" +
			"    --rate-type fixed --rate 3,15 --debit Checking\n" +
			"  fui loan add Mutuo --kind mortgage --term-months 240 --rate-type fixed --rate 3,15 \\\n" +
			"    --debit Checking --principal 200000 --import schedule.fui.jsonl",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoanAdd(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("kind", "", "mortgage or loan (required)")
	cmd.Flags().String("principal", "", "capital disbursed, positive (required)")
	cmd.Flags().String("disbursed", "", "disbursement date (default: today)")
	cmd.Flags().Int("term-months", 0, "original duration in months (required)")
	cmd.Flags().String("rate-type", "", "fixed, variable or mixed (required)")
	cmd.Flags().String("rate", "", "annual nominal rate as a percentage, e.g. 3,15 (required)")
	cmd.Flags().String("debit", "", "account the installments are debited from (required)")
	cmd.Flags().String("installment", "", "bank-stated installment amount, overrides the computed one")
	cmd.Flags().Bool("computed", false, "compute a French amortization schedule (default)")
	cmd.Flags().String("import", "", "fui/1 JSONL file of installment records to use as the schedule, instead of computing one")
	return cmd
}

func runLoanAdd(cmd *cobra.Command, deps *Deps, name string) error {
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
	req, err := loanAddRequest(cmd, deps, name)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Loans.PlanAdd(ctx, enabled, req)
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		info, err := deps.Loans.ApplyAdd(ctx, plan)
		if err != nil {
			return err
		}
		view, err := buildLoanView(ctx, deps, info, nil)
		if err != nil {
			return err
		}
		return render.WriteLoan(deps.Stdout, rt.format, deps.Locale(), view)
	})
}

func loanAddRequest(cmd *cobra.Command, deps *Deps, name string) (lending.AddRequest, error) {
	computed, _ := cmd.Flags().GetBool("computed")
	importFile, _ := cmd.Flags().GetString("import")
	if computed && importFile != "" {
		return lending.AddRequest{}, newUsageError(fmt.Errorf("--computed and --import are mutually exclusive"))
	}
	kind, _ := cmd.Flags().GetString("kind")
	principal, _ := cmd.Flags().GetString("principal")
	disbursed, _ := cmd.Flags().GetString("disbursed")
	termMonths, _ := cmd.Flags().GetInt("term-months")
	rateType, _ := cmd.Flags().GetString("rate-type")
	rate, _ := cmd.Flags().GetString("rate")
	debit, _ := cmd.Flags().GetString("debit")
	installment, _ := cmd.Flags().GetString("installment")
	req := lending.AddRequest{
		Name: name, Kind: kind, Principal: principal, Disbursed: disbursed, TermMonths: termMonths,
		RateType: rateType, Rate: rate, Debit: debit, Installment: installment,
	}
	if importFile == "" {
		return req, nil
	}
	currency, err := resolveDebitCurrency(cmd.Context(), deps, debit)
	if err != nil {
		return lending.AddRequest{}, err
	}
	imported, err := parseInstallmentsFile(importFile, currency)
	if err != nil {
		return lending.AddRequest{}, err
	}
	req.Imported = imported
	return req, nil
}
