package memstore

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

type rules struct{ *repos }

func (r rules) Insert(_ context.Context, rule domain.Rule) (domain.Rule, error) {
	if err := r.writable(); err != nil {
		return domain.Rule{}, err
	}
	norm, err := domain.NewRule(rule)
	if err != nil {
		return domain.Rule{}, err
	}
	key := ruleKey(norm.Name)
	if _, ok := r.st.Rules[key]; ok {
		return domain.Rule{}, exists("rule", norm.Name)
	}
	r.st.Rules[key] = norm
	return norm, nil
}

func (r rules) Update(_ context.Context, rule domain.Rule) error {
	if err := r.writable(); err != nil {
		return err
	}
	norm, err := domain.NewRule(rule)
	if err != nil {
		return err
	}
	key := ruleKey(norm.Name)
	if _, ok := r.st.Rules[key]; !ok {
		return notFound("rule", norm.Name)
	}
	r.st.Rules[key] = norm
	return nil
}

func (r rules) Delete(_ context.Context, name string) error {
	if err := r.writable(); err != nil {
		return err
	}
	key := ruleKey(name)
	if _, ok := r.st.Rules[key]; !ok {
		return notFound("rule", name)
	}
	delete(r.st.Rules, key)
	return nil
}

func (r rules) Get(_ context.Context, name string) (domain.Rule, error) {
	rule, ok := r.st.Rules[ruleKey(name)]
	if !ok {
		return domain.Rule{}, notFound("rule", name)
	}
	return rule, nil
}

func (r rules) List(context.Context) ([]domain.Rule, error) {
	out := slices.Collect(func(yield func(domain.Rule) bool) {
		for _, rule := range r.st.Rules {
			if !yield(rule) {
				return
			}
		}
	})
	sortRulesByName(out)
	return out, nil
}

func (r rules) ListEnabledByPriority(context.Context) ([]domain.Rule, error) {
	out := slices.Collect(func(yield func(domain.Rule) bool) {
		for _, rule := range r.st.Rules {
			if rule.Enabled && !yield(rule) {
				return
			}
		}
	})
	slices.SortFunc(out, func(a, b domain.Rule) int {
		return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)))
	})
	return out, nil
}

func sortRulesByName(rules []domain.Rule) {
	slices.SortFunc(rules, func(a, b domain.Rule) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}

func ruleKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
