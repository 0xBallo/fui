package cli

import (
	"context"
	"fmt"
	"math"
	"strings"

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
	cmd.Flags().String("account", "", "account name (optional; account conditions are not evaluated without it)")
	cmd.Flags().String("amount", "", "transaction amount (optional; amount conditions are not evaluated without it)")
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
	ctx := cmd.Context()
	var res domain.RuleApplyResult
	if lenientRuleTest(cmd, args) {
		res, err = lenientRuleTestResult(cmd, deps, args[0])
	} else {
		var req rules.TestRequest
		if req, err = ruleTestRequest(cmd, deps, args); err == nil {
			res, err = deps.Rules.Test(ctx, req)
		}
	}
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

// lenientRuleTest reports whether the hypothetical transaction lacks the account or the amount.
// The rules engine needs both, so that case is evaluated here on text/payee conditions only.
func lenientRuleTest(cmd *cobra.Command, args []string) bool {
	if txID, _ := cmd.Flags().GetInt64("tx"); txID != 0 || len(args) != 1 {
		return false
	}
	account, _ := cmd.Flags().GetString("account")
	amount, _ := cmd.Flags().GetString("amount")
	return account == "" || amount == ""
}

// lenientRuleTestResult applies the enabled rules with the conditions on the missing account
// and/or amount switched off, and tells the user which ones were not evaluated.
func lenientRuleTestResult(cmd *cobra.Command, deps *Deps, text string) (domain.RuleApplyResult, error) {
	account, _ := cmd.Flags().GetString("account")
	amount, _ := cmd.Flags().GetString("amount")
	if account == "" && amount != "" {
		return domain.RuleApplyResult{}, missingFieldError("account")
	}
	ctx := cmd.Context()
	tx, err := lenientTx(ctx, deps, account, text)
	if err != nil {
		return domain.RuleApplyResult{}, err
	}
	all, err := deps.Rules.List(ctx)
	if err != nil {
		return domain.RuleApplyResult{}, err
	}
	var skipped []string
	if account == "" {
		skipped = append(skipped, "account")
	}
	if amount == "" {
		skipped = append(skipped, "amount")
	}
	for i := range all {
		if account == "" {
			all[i].Conditions.AccountIDs = nil
		}
		if amount == "" {
			all[i].Conditions.Amount = domain.RuleAmountRange{}
		}
	}
	res, err := domain.Apply(all, tx, domain.NormalizeDescription(text))
	if err != nil {
		return domain.RuleApplyResult{}, err
	}
	if _, err := fmt.Fprintf(deps.Stderr, "note: %s conditions not evaluated (not given); only text/payee conditions were checked\n",
		strings.Join(skipped, "/")); err != nil {
		return domain.RuleApplyResult{}, fmt.Errorf("write rule-test note: %w", err)
	}
	return res, nil
}

// lenientTx builds the hypothetical transaction: the named account when given, else a
// placeholder whose account conditions the caller has switched off. The amount is always zero.
func lenientTx(ctx context.Context, deps *Deps, account, text string) (domain.Transaction, error) {
	tx := domain.Transaction{AccountID: 1, Description: text}
	cur, err := domain.ParseCurrency("EUR")
	if err != nil {
		return tx, fmt.Errorf("placeholder currency: %w", err)
	}
	if account != "" {
		accts, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{IncludeArchived: true})
		if err != nil {
			return tx, err
		}
		names := make([]string, len(accts))
		for i := range accts {
			names[i] = accts[i].Account.Name
		}
		i, err := domain.Resolve(account, names)
		if err != nil {
			return tx, fmt.Errorf("account: %w", err)
		}
		tx.AccountID, cur = accts[i].Account.ID, accts[i].Account.Currency
	}
	if tx.Amount, err = domain.NewMoney(0, cur); err != nil {
		return tx, fmt.Errorf("amount: %w", err)
	}
	return tx, nil
}
