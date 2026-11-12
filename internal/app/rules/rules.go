// Package rules holds the use cases of categorization rules: CRUD, testing a rule set against a
// text or a transaction, and applying it to existing transactions. Rules belong to the core module.
package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// EntityRule is the Change.Entity of a rule.
const EntityRule = "rule"

// Service runs the rule use cases over a Store.
type Service struct {
	store  app.Store
	backup app.Backuper
}

// New builds a Service.
func New(store app.Store, backup app.Backuper) *Service {
	return &Service{store: store, backup: backup}
}

// ConditionsSpec is the user-typed form of domain.RuleConditions; Accounts are names. Amounts
// are absolute minor units, rules being currency-agnostic.
type ConditionsSpec struct {
	DescriptionContains string
	DescriptionEquals   string
	MinAbsMinor         *int64
	MaxAbsMinor         *int64
	Sign                string // any, inflow or outflow; empty means any
	Accounts            []string
}

// ActionsSpec is the user-typed form of domain.RuleActions: names of an existing category,
// payee and tags.
type ActionsSpec struct {
	Category string
	Payee    string
	Tags     []string
}

// AddRequest describes a new rule. Disabled creates it switched off.
type AddRequest struct {
	Name       string
	Priority   int
	Disabled   bool
	Conditions ConditionsSpec
	Actions    ActionsSpec
}

// RulePlan is the plan to store Rule (insert or replace).
type RulePlan struct {
	app.Plan
	Rule domain.Rule
}

// PlanAdd validates req and plans the rule creation.
func (s *Service) PlanAdd(ctx context.Context, req AddRequest) (RulePlan, error) {
	var out RulePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		rule, err := build(ctx, r, req.Name, req.Priority, !req.Disabled, req.Conditions, req.Actions)
		if err != nil {
			return err
		}
		if _, err := r.Rules().Get(ctx, rule.Name); err == nil {
			return &domain.Error{Code: app.CodeAlreadyExists, Message: fmt.Sprintf("rule %q already exists", rule.Name)}
		}
		out = RulePlan{Plan: change(app.OpCreate, rule), Rule: rule}
		return nil
	})
	return out, err
}

// ApplyAdd stores the rule of the plan.
func (s *Service) ApplyAdd(ctx context.Context, p RulePlan) (domain.Rule, error) {
	var out domain.Rule
	err := s.store.WithTx(ctx, func(r app.Repos) (err error) {
		out, err = r.Rules().Insert(ctx, p.Rule)
		return err
	})
	if err != nil {
		return domain.Rule{}, fmt.Errorf("add rule: %w", err)
	}
	return out, nil
}

// EditRequest changes the non-nil fields of the rule Name; a given Conditions or Actions replaces
// the whole block.
type EditRequest struct {
	Name       string
	Priority   *int
	Enabled    *bool
	Conditions *ConditionsSpec
	Actions    *ActionsSpec
}

// PlanEdit plans the edit of an existing rule.
func (s *Service) PlanEdit(ctx context.Context, req EditRequest) (RulePlan, error) {
	if req.Priority == nil && req.Enabled == nil && req.Conditions == nil && req.Actions == nil {
		return RulePlan{}, &domain.Error{
			Code: domain.CodeMissingField, Message: "nothing to change",
			Details: map[string]any{"field": "changes"},
		}
	}
	var out RulePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		cur, err := r.Rules().Get(ctx, req.Name)
		if err != nil {
			return fmt.Errorf("get rule %q: %w", req.Name, err)
		}
		if req.Priority != nil {
			cur.Priority = *req.Priority
		}
		if req.Enabled != nil {
			cur.Enabled = *req.Enabled
		}
		if req.Conditions != nil {
			if cur.Conditions, err = conditions(ctx, r, *req.Conditions); err != nil {
				return err
			}
		}
		if req.Actions != nil {
			if cur.Actions, err = actions(ctx, r, *req.Actions); err != nil {
				return err
			}
		}
		if cur, err = domain.NewRule(cur); err != nil {
			return err
		}
		out = RulePlan{Plan: change(app.OpUpdate, cur), Rule: cur}
		return nil
	})
	return out, err
}

// ApplyEdit stores the rule of the plan.
func (s *Service) ApplyEdit(ctx context.Context, p RulePlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Rules().Update(ctx, p.Rule) })
}

// PlanRemove plans the removal of the rule called name.
func (s *Service) PlanRemove(ctx context.Context, name string) (RulePlan, error) {
	var out RulePlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		rule, err := r.Rules().Get(ctx, name)
		if err != nil {
			return fmt.Errorf("get rule %q: %w", name, err)
		}
		out = RulePlan{Plan: change(app.OpDelete, rule), Rule: rule}
		return nil
	})
	return out, err
}

// ApplyRemove deletes the rule of the plan.
func (s *Service) ApplyRemove(ctx context.Context, p RulePlan) error {
	return s.write(ctx, func(r app.Repos) error { return r.Rules().Delete(ctx, p.Rule.Name) })
}

// List returns every rule ordered by name.
func (s *Service) List(ctx context.Context) ([]domain.Rule, error) {
	var out []domain.Rule
	err := s.store.Read(ctx, func(r app.Repos) (err error) {
		out, err = r.Rules().List(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	return out, nil
}

func (s *Service) write(ctx context.Context, fn func(app.Repos) error) error {
	if err := s.store.WithTx(ctx, fn); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}

func change(op app.Op, r domain.Rule) app.Plan {
	return app.Plan{Changes: []app.Change{{
		Op: op, Entity: EntityRule, Summary: fmt.Sprintf("%s rule %q", op, r.Name),
		Fields: map[string]any{
			"name": r.Name, "priority": r.Priority, "enabled": r.Enabled,
			"conditions": r.Conditions, "actions": r.Actions,
		},
	}}}
}

func build(ctx context.Context, r app.Repos, name string, prio int, enabled bool, c ConditionsSpec, a ActionsSpec) (domain.Rule, error) {
	conds, err := conditions(ctx, r, c)
	if err != nil {
		return domain.Rule{}, err
	}
	acts, err := actions(ctx, r, a)
	if err != nil {
		return domain.Rule{}, err
	}
	return domain.NewRule(domain.Rule{Name: name, Priority: prio, Enabled: enabled, Conditions: conds, Actions: acts})
}

func conditions(ctx context.Context, r app.Repos, c ConditionsSpec) (domain.RuleConditions, error) {
	out := domain.RuleConditions{
		DescriptionContains: c.DescriptionContains, DescriptionEquals: c.DescriptionEquals,
		Amount: domain.RuleAmountRange{MinAbsMinor: c.MinAbsMinor, MaxAbsMinor: c.MaxAbsMinor, Sign: domain.RuleAmountSign(strings.ToLower(c.Sign))},
	}
	if len(c.Accounts) == 0 {
		return out, nil
	}
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return out, fmt.Errorf("list accounts: %w", err)
	}
	names := make([]string, len(accts))
	for i, a := range accts {
		names[i] = a.Name
	}
	for _, ref := range c.Accounts {
		i, err := domain.Resolve(ref, names)
		if err != nil {
			return out, fmt.Errorf("account: %w", err)
		}
		out.AccountIDs = append(out.AccountIDs, accts[i].ID)
	}
	return out, nil
}

func actions(ctx context.Context, r app.Repos, a ActionsSpec) (domain.RuleActions, error) {
	var out domain.RuleActions
	if a.Category != "" {
		cats, err := r.Categories().List(ctx)
		if err != nil {
			return out, fmt.Errorf("list categories: %w", err)
		}
		names := make([]string, len(cats))
		for i, c := range cats {
			names[i] = c.Name
		}
		i, err := domain.Resolve(a.Category, names)
		if err != nil {
			return out, fmt.Errorf("category: %w", err)
		}
		out.CategoryID = cats[i].ID
	}
	if a.Payee != "" {
		ps, err := r.Payees().List(ctx)
		if err != nil {
			return out, fmt.Errorf("list payees: %w", err)
		}
		names := make([]string, len(ps))
		for i, p := range ps {
			names[i] = p.Name
		}
		i, err := domain.Resolve(a.Payee, names)
		if err != nil {
			return out, fmt.Errorf("payee: %w", err)
		}
		out.PayeeID = ps[i].ID
	}
	if len(a.Tags) > 0 {
		ts, err := r.Tags().List(ctx)
		if err != nil {
			return out, fmt.Errorf("list tags: %w", err)
		}
		names := make([]string, len(ts))
		for i, t := range ts {
			names[i] = t.Name
		}
		for _, ref := range a.Tags {
			i, err := domain.Resolve(ref, names)
			if err != nil {
				return out, fmt.Errorf("tag: %w", err)
			}
			out.TagIDs = append(out.TagIDs, ts[i].ID)
		}
	}
	return out, nil
}
