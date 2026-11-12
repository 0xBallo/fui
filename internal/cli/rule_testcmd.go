package cli

import (
	"context"
	"fmt"
	"math"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

func newRuleTestCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "test [TEXT]",
		Short:   "Test which enabled rules match a transaction",
		Example: "  fui rule test \"AMAZON\" --account Checking --amount -12,50\n  fui rule test --tx 42",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuleTest(cmd, deps, args)
		},
	}
	cmd.Flags().Int64("tx", 0, "existing transaction id to test")
	cmd.Flags().String("account", "", "account name (required unless --tx is set)")
	cmd.Flags().String("amount", "", "transaction amount (required unless --tx is set)")
	return cmd
}

func runRuleTest(cmd *cobra.Command, deps *Deps, args []string) error {
	if err := requireRules(deps); err != nil {
		return err
	}
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	req, err := ruleTestRequest(cmd, deps, args)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	res, err := deps.Rules.Test(ctx, req)
	if err != nil {
		return err
	}
	lookup, err := loadRuleLookup(ctx, deps)
	if err != nil {
		return err
	}
	return render.WriteRuleTestResult(deps.Stdout, rt.format, res, lookup)
}

func ruleTestRequest(cmd *cobra.Command, deps *Deps, args []string) (rules.TestRequest, error) {
	txID, _ := cmd.Flags().GetInt64("tx")
	if txID != 0 {
		if len(args) > 0 {
			return rules.TestRequest{}, newUsageError(fmt.Errorf("TEXT cannot be used with --tx"))
		}
		return rules.TestRequest{TxID: domain.TransactionID(txID)}, nil
	}
	if len(args) != 1 {
		return rules.TestRequest{}, newUsageError(fmt.Errorf("accepts 1 arg(s), received %d", len(args)))
	}
	account, _ := cmd.Flags().GetString("account")
	amount, _ := cmd.Flags().GetString("amount")
	if account == "" || amount == "" {
		return rules.TestRequest{}, missingFieldError("account/amount")
	}
	cur, err := currencyOfAccount(cmd.Context(), deps, account)
	if err != nil {
		return rules.TestRequest{}, err
	}
	m, err := domain.ParseAmount(amount, deps.Locale(), cur)
	if err != nil {
		return rules.TestRequest{}, fmt.Errorf("amount: %w", err)
	}
	if m.Minor() == math.MinInt64 {
		return rules.TestRequest{}, fmt.Errorf("amount: invalid value")
	}
	return rules.TestRequest{Text: args[0], Account: account, AmountMinor: m.Minor()}, nil
}

func currencyOfAccount(ctx context.Context, deps *Deps, ref string) (domain.Currency, error) {
	page, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{IncludeArchived: true})
	if err != nil {
		return domain.Currency{}, err
	}
	names := make([]string, len(page))
	for i := range page {
		names[i] = page[i].Account.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Currency{}, fmt.Errorf("account: %w", err)
	}
	return page[i].Account.Currency, nil
}
