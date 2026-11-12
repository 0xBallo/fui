package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type ruleRepo struct {
	repos *repos
}

func (r ruleRepo) Insert(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Rule{}, err
	}
	params, err := toInsertRuleParams(rule)
	if err != nil {
		return domain.Rule{}, err
	}
	row, err := r.repos.queries.InsertRule(ctx, params)
	if err != nil {
		return domain.Rule{}, mapConstraint(err)
	}
	return toRule(row)
}

func (r ruleRepo) Update(ctx context.Context, rule domain.Rule) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	params, err := toUpdateRuleParams(rule)
	if err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateRule(ctx, params)
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("rule %q: %w", rule.Name, domain.ErrNotFound)
	}
	return nil
}

func (r ruleRepo) Delete(ctx context.Context, name string) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteRule(ctx, name)
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("rule %q: %w", name, domain.ErrNotFound)
	}
	return nil
}

func (r ruleRepo) Get(ctx context.Context, name string) (domain.Rule, error) {
	row, err := r.repos.queries.GetRule(ctx, name)
	if err != nil {
		return domain.Rule{}, mapNotFound(mapSQLError(err), "get rule")
	}
	return toRule(row)
}

func (r ruleRepo) List(ctx context.Context) ([]domain.Rule, error) {
	rows, err := r.repos.queries.ListRules(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Rule, 0, len(rows))
	for _, row := range rows {
		rule, convErr := toRule(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, rule)
	}
	return out, nil
}

func (r ruleRepo) ListEnabledByPriority(ctx context.Context) ([]domain.Rule, error) {
	rows, err := r.repos.queries.ListEnabledRulesByPriority(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Rule, 0, len(rows))
	for _, row := range rows {
		rule, convErr := toRule(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, rule)
	}
	return out, nil
}

func toInsertRuleParams(rule domain.Rule) (gen.InsertRuleParams, error) {
	norm, condJSON, actJSON, err := normalizeAndEncodeRule(rule)
	if err != nil {
		return gen.InsertRuleParams{}, err
	}
	return gen.InsertRuleParams{
		Name:       norm.Name,
		Priority:   int64(norm.Priority),
		Enabled:    boolToInt64(norm.Enabled),
		Conditions: condJSON,
		Actions:    actJSON,
	}, nil
}

func toUpdateRuleParams(rule domain.Rule) (gen.UpdateRuleParams, error) {
	norm, condJSON, actJSON, err := normalizeAndEncodeRule(rule)
	if err != nil {
		return gen.UpdateRuleParams{}, err
	}
	return gen.UpdateRuleParams{
		Name:       norm.Name,
		Priority:   int64(norm.Priority),
		Enabled:    boolToInt64(norm.Enabled),
		Conditions: condJSON,
		Actions:    actJSON,
	}, nil
}

func normalizeAndEncodeRule(rule domain.Rule) (domain.Rule, string, string, error) {
	norm, err := domain.NewRule(rule)
	if err != nil {
		return domain.Rule{}, "", "", err
	}
	condJSON, err := encodeJSON(norm.Conditions)
	if err != nil {
		return domain.Rule{}, "", "", fmt.Errorf("encode rule conditions: %w", err)
	}
	actJSON, err := encodeJSON(norm.Actions)
	if err != nil {
		return domain.Rule{}, "", "", fmt.Errorf("encode rule actions: %w", err)
	}
	return norm, condJSON, actJSON, nil
}

func toRule(row gen.Rule) (domain.Rule, error) {
	var conditions domain.RuleConditions
	if err := json.Unmarshal([]byte(row.Conditions), &conditions); err != nil {
		return domain.Rule{}, fmt.Errorf("decode rule %q conditions: %w", row.Name, err)
	}
	var actions domain.RuleActions
	if err := json.Unmarshal([]byte(row.Actions), &actions); err != nil {
		return domain.Rule{}, fmt.Errorf("decode rule %q actions: %w", row.Name, err)
	}

	return domain.NewRule(domain.Rule{
		Name:       row.Name,
		Priority:   int(row.Priority),
		Enabled:    row.Enabled != 0,
		Conditions: conditions,
		Actions:    actions,
	})
}
