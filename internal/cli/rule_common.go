package cli

import (
	"context"
	"fmt"
	"math"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

func parseRuleAmount(deps *Deps, raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	cur, err := deps.BaseCurrency()
	if err != nil {
		return nil, err
	}
	m, err := domain.ParseAmount(raw, deps.Locale(), cur)
	if err != nil {
		return nil, fmt.Errorf("amount: %w", err)
	}
	v := m.Minor()
	if v == math.MinInt64 {
		return nil, fmt.Errorf("amount: invalid value")
	}
	if v < 0 {
		v = -v
	}
	return &v, nil
}

func ruleConditionsFromFlags(cmd flagReader, deps *Deps) (rules.ConditionsSpec, error) {
	minIn, _ := cmd.GetString("min")
	maxIn, _ := cmd.GetString("max")
	minAbs, err := parseRuleAmount(deps, minIn)
	if err != nil {
		return rules.ConditionsSpec{}, fmt.Errorf("min: %w", err)
	}
	maxAbs, err := parseRuleAmount(deps, maxIn)
	if err != nil {
		return rules.ConditionsSpec{}, fmt.Errorf("max: %w", err)
	}
	contains, _ := cmd.GetString("contains")
	equals, _ := cmd.GetString("equals")
	sign, _ := cmd.GetString("sign")
	accounts, _ := cmd.GetStringArray("account")
	return rules.ConditionsSpec{
		DescriptionContains: contains,
		DescriptionEquals:   equals,
		MinAbsMinor:         minAbs,
		MaxAbsMinor:         maxAbs,
		Sign:                sign,
		Accounts:            accounts,
	}, nil
}

func ruleActionsFromFlags(cmd flagReader) rules.ActionsSpec {
	category, _ := cmd.GetString("category")
	payee, _ := cmd.GetString("payee")
	tags, _ := cmd.GetStringArray("tag")
	return rules.ActionsSpec{Category: category, Payee: payee, Tags: tags}
}

type flagReader interface {
	GetString(name string) (string, error)
	GetStringArray(name string) ([]string, error)
}

func loadRuleLookup(ctx context.Context, deps *Deps) (render.RuleLookup, error) {
	accts, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{})
	if err != nil {
		return render.RuleLookup{}, err
	}
	cats, err := deps.Service.ListCategories(ctx)
	if err != nil {
		return render.RuleLookup{}, err
	}
	payees, err := deps.Service.ListPayees(ctx)
	if err != nil {
		return render.RuleLookup{}, err
	}
	tags, err := deps.Service.ListTags(ctx)
	if err != nil {
		return render.RuleLookup{}, err
	}
	return render.NewRuleLookup(accts, cats, payees, tags), nil
}
