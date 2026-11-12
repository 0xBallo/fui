package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
)

// RuleLookup resolves ids to names while rendering rules and rule-application previews.
type RuleLookup struct {
	accounts   map[domain.AccountID]string
	categories map[domain.CategoryID]string
	payees     map[domain.PayeeID]string
	tags       map[domain.TagID]string
}

// NewRuleLookup builds RuleLookup from app/domain rows.
func NewRuleLookup(accts []app.AccountBalance, cats []domain.Category, payees []domain.Payee, tags []domain.Tag) RuleLookup {
	out := RuleLookup{
		accounts:   make(map[domain.AccountID]string, len(accts)),
		categories: make(map[domain.CategoryID]string, len(cats)),
		payees:     make(map[domain.PayeeID]string, len(payees)),
		tags:       make(map[domain.TagID]string, len(tags)),
	}
	for _, a := range accts {
		out.accounts[a.Account.ID] = a.Account.Name
	}
	for _, c := range cats {
		out.categories[c.ID] = c.Name
	}
	for _, p := range payees {
		out.payees[p.ID] = p.Name
	}
	for _, t := range tags {
		out.tags[t.ID] = t.Name
	}
	return out
}

// RuleDTO is the --json shape of one rule.
type RuleDTO struct {
	Name       string        `json:"name"`
	Priority   int           `json:"priority"`
	Enabled    bool          `json:"enabled"`
	Conditions RuleCondDTO   `json:"conditions"`
	Actions    RuleActionDTO `json:"actions"`
}

// RuleCondDTO is the --json conditions shape.
type RuleCondDTO struct {
	DescriptionContains string   `json:"description_contains,omitempty"`
	DescriptionEquals   string   `json:"description_equals,omitempty"`
	MinAbsMinor         *int64   `json:"min_abs_minor,omitempty"`
	MaxAbsMinor         *int64   `json:"max_abs_minor,omitempty"`
	Sign                string   `json:"sign,omitempty"`
	AccountIDs          []int64  `json:"account_ids,omitempty"`
	Accounts            []string `json:"accounts,omitempty"`
}

// RuleActionDTO is the --json actions shape.
type RuleActionDTO struct {
	CategoryID int64    `json:"category_id,omitempty"`
	Category   string   `json:"category,omitempty"`
	PayeeID    int64    `json:"payee_id,omitempty"`
	Payee      string   `json:"payee,omitempty"`
	TagIDs     []int64  `json:"tag_ids,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func ruleDTO(r domain.Rule, lookup RuleLookup) RuleDTO {
	out := RuleDTO{
		Name:     r.Name,
		Priority: r.Priority,
		Enabled:  r.Enabled,
		Conditions: RuleCondDTO{
			DescriptionContains: r.Conditions.DescriptionContains,
			DescriptionEquals:   r.Conditions.DescriptionEquals,
			MinAbsMinor:         r.Conditions.Amount.MinAbsMinor,
			MaxAbsMinor:         r.Conditions.Amount.MaxAbsMinor,
			Sign:                string(r.Conditions.Amount.Sign),
		},
		Actions: RuleActionDTO{
			CategoryID: int64(r.Actions.CategoryID),
			PayeeID:    int64(r.Actions.PayeeID),
		},
	}
	for _, id := range r.Conditions.AccountIDs {
		out.Conditions.AccountIDs = append(out.Conditions.AccountIDs, int64(id))
		if name := lookup.accounts[id]; name != "" {
			out.Conditions.Accounts = append(out.Conditions.Accounts, name)
		}
	}
	if out.Actions.CategoryID != 0 {
		out.Actions.Category = lookup.categories[r.Actions.CategoryID]
	}
	if out.Actions.PayeeID != 0 {
		out.Actions.Payee = lookup.payees[r.Actions.PayeeID]
	}
	for _, id := range r.Actions.TagIDs {
		out.Actions.TagIDs = append(out.Actions.TagIDs, int64(id))
		if name := lookup.tags[id]; name != "" {
			out.Actions.Tags = append(out.Actions.Tags, name)
		}
	}
	return out
}

func ruleRows(items []domain.Rule, lookup RuleLookup) Rows {
	t := Rows{Header: []string{"NAME", "PRIORITY", "ENABLED", "CONDITIONS", "ACTIONS"}}
	for _, r := range items {
		t.Body = append(t.Body, []string{
			r.Name,
			strconv.Itoa(r.Priority),
			boolCell(r.Enabled),
			ruleConditionsText(r, lookup),
			ruleActionsText(r, lookup),
		})
	}
	return t
}

func ruleConditionsText(r domain.Rule, lookup RuleLookup) string {
	var parts []string
	if r.Conditions.DescriptionContains != "" {
		parts = append(parts, "contains="+r.Conditions.DescriptionContains)
	}
	if r.Conditions.DescriptionEquals != "" {
		parts = append(parts, "equals="+r.Conditions.DescriptionEquals)
	}
	if r.Conditions.Amount.MinAbsMinor != nil {
		parts = append(parts, fmt.Sprintf("min=%d", *r.Conditions.Amount.MinAbsMinor))
	}
	if r.Conditions.Amount.MaxAbsMinor != nil {
		parts = append(parts, fmt.Sprintf("max=%d", *r.Conditions.Amount.MaxAbsMinor))
	}
	if r.Conditions.Amount.Sign != "" && r.Conditions.Amount.Sign != domain.RuleSignAny {
		parts = append(parts, "sign="+string(r.Conditions.Amount.Sign))
	}
	if len(r.Conditions.AccountIDs) > 0 {
		var names []string
		for _, id := range r.Conditions.AccountIDs {
			if name := lookup.accounts[id]; name != "" {
				names = append(names, name)
				continue
			}
			names = append(names, strconv.FormatInt(int64(id), 10))
		}
		parts = append(parts, "accounts="+strings.Join(names, ","))
	}
	return strings.Join(parts, "; ")
}

func ruleActionsText(r domain.Rule, lookup RuleLookup) string {
	var parts []string
	if r.Actions.CategoryID != 0 {
		if name := lookup.categories[r.Actions.CategoryID]; name != "" {
			parts = append(parts, "category="+name)
		} else {
			parts = append(parts, "category="+strconv.FormatInt(int64(r.Actions.CategoryID), 10))
		}
	}
	if r.Actions.PayeeID != 0 {
		if name := lookup.payees[r.Actions.PayeeID]; name != "" {
			parts = append(parts, "payee="+name)
		} else {
			parts = append(parts, "payee="+strconv.FormatInt(int64(r.Actions.PayeeID), 10))
		}
	}
	if len(r.Actions.TagIDs) > 0 {
		var tags []string
		for _, id := range r.Actions.TagIDs {
			if name := lookup.tags[id]; name != "" {
				tags = append(tags, name)
			} else {
				tags = append(tags, strconv.FormatInt(int64(id), 10))
			}
		}
		parts = append(parts, "tags="+strings.Join(tags, ","))
	}
	return strings.Join(parts, "; ")
}

// WriteRuleList renders rules.
func WriteRuleList(w io.Writer, f Format, items []domain.Rule, lookup RuleLookup) error {
	if f == JSON {
		dtos := make([]RuleDTO, len(items))
		for i, r := range items {
			dtos[i] = ruleDTO(r, lookup)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, ruleRows(items, lookup))
	}
	return WriteTable(w, ruleRows(items, lookup), false)
}

// WriteRule renders one rule.
func WriteRule(w io.Writer, f Format, item domain.Rule, lookup RuleLookup) error {
	if f == JSON {
		return WriteJSON(w, ruleDTO(item, lookup))
	}
	return WriteRuleList(w, f, []domain.Rule{item}, lookup)
}

// RuleTestResultDTO is the --json shape of `rule test`.
type RuleTestResultDTO struct {
	CategoryID   int64    `json:"category_id,omitempty"`
	Category     string   `json:"category,omitempty"`
	CategoryRule string   `json:"category_rule,omitempty"`
	PayeeID      int64    `json:"payee_id,omitempty"`
	Payee        string   `json:"payee,omitempty"`
	PayeeRule    string   `json:"payee_rule,omitempty"`
	TagIDs       []int64  `json:"tag_ids,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	TagsRule     string   `json:"tags_rule,omitempty"`
}

func ruleTestDTO(in domain.RuleApplyResult, lookup RuleLookup) RuleTestResultDTO {
	out := RuleTestResultDTO{
		CategoryID:   int64(in.CategoryID),
		CategoryRule: in.CategoryRule,
		PayeeID:      int64(in.PayeeID),
		PayeeRule:    in.PayeeRule,
		TagsRule:     in.TagsRule,
	}
	if in.CategoryID != 0 {
		out.Category = lookup.categories[in.CategoryID]
	}
	if in.PayeeID != 0 {
		out.Payee = lookup.payees[in.PayeeID]
	}
	for _, id := range in.TagIDs {
		out.TagIDs = append(out.TagIDs, int64(id))
		if name := lookup.tags[id]; name != "" {
			out.Tags = append(out.Tags, name)
		}
	}
	return out
}

func ruleTestRows(res domain.RuleApplyResult, lookup RuleLookup) Rows {
	dto := ruleTestDTO(res, lookup)
	t := Rows{Header: []string{"FIELD", "VALUE", "RULE"}}
	t.Body = append(t.Body, []string{"category", orEmpty(dto.Category, dto.CategoryID), dto.CategoryRule})
	t.Body = append(t.Body, []string{"payee", orEmpty(dto.Payee, dto.PayeeID), dto.PayeeRule})
	t.Body = append(t.Body, []string{"tags", strings.Join(dto.Tags, ","), dto.TagsRule})
	return t
}

func orEmpty(name string, id int64) string {
	if name != "" {
		return name
	}
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

// WriteRuleTestResult renders `rule test`.
func WriteRuleTestResult(w io.Writer, f Format, res domain.RuleApplyResult, lookup RuleLookup) error {
	if f == JSON {
		return WriteJSON(w, ruleTestDTO(res, lookup))
	}
	if f == CSV {
		return WriteCSV(w, ruleTestRows(res, lookup))
	}
	return WriteTable(w, ruleTestRows(res, lookup), false)
}

// RuleApplyEffectDTO is one transaction effect of `rule apply`.
type RuleApplyEffectDTO struct {
	ID           int64   `json:"id"`
	CategoryID   int64   `json:"category_id,omitempty"`
	CategoryRule string  `json:"category_rule,omitempty"`
	PayeeID      int64   `json:"payee_id,omitempty"`
	PayeeRule    string  `json:"payee_rule,omitempty"`
	TagIDs       []int64 `json:"tag_ids,omitempty"`
	TagsRule     string  `json:"tags_rule,omitempty"`
}

// RuleApplyPlanDTO is the --json shape of `rule apply` preview/result.
type RuleApplyPlanDTO struct {
	Applied           []RuleApplyEffectDTO `json:"applied"`
	SkippedReconciled int                  `json:"skipped_reconciled"`
	Plan              PlanDTO              `json:"plan"`
}

func ruleApplyDTO(plan rules.ApplyPlan) RuleApplyPlanDTO {
	out := RuleApplyPlanDTO{
		SkippedReconciled: plan.Skipped,
		Plan:              planDTO(plan.Plan),
		Applied:           make([]RuleApplyEffectDTO, 0, len(plan.Effects)),
	}
	for _, e := range plan.Effects {
		row := RuleApplyEffectDTO{
			ID:           int64(e.Tx.ID),
			CategoryID:   int64(e.Tx.CategoryID),
			CategoryRule: e.Result.CategoryRule,
			PayeeID:      int64(e.Tx.PayeeID),
			PayeeRule:    e.Result.PayeeRule,
			TagsRule:     e.Result.TagsRule,
		}
		for _, id := range e.TagIDs {
			row.TagIDs = append(row.TagIDs, int64(id))
		}
		out.Applied = append(out.Applied, row)
	}
	return out
}

func ruleApplyRows(plan rules.ApplyPlan) Rows {
	t := Rows{Header: []string{"ID", "CATEGORY RULE", "PAYEE RULE", "TAGS RULE"}}
	for _, e := range plan.Effects {
		t.Body = append(t.Body, []string{
			strconv.FormatInt(int64(e.Tx.ID), 10),
			e.Result.CategoryRule,
			e.Result.PayeeRule,
			e.Result.TagsRule,
		})
	}
	return t
}

// WriteRuleApplyPlan renders `rule apply` preview/result.
func WriteRuleApplyPlan(w io.Writer, f Format, plan rules.ApplyPlan) error {
	if f == JSON {
		return WriteJSON(w, ruleApplyDTO(plan))
	}
	if f == CSV {
		return WriteCSV(w, ruleApplyRows(plan))
	}
	if err := WriteTable(w, ruleApplyRows(plan), false); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Applied to %d transaction(s), skipped reconciled: %d\n", len(plan.Effects), plan.Skipped); err != nil {
		return fmt.Errorf("write rule-apply summary: %w", err)
	}
	if plan.Bulk {
		if _, err := fmt.Fprintln(w, "This is a bulk change: it needs --yes or a confirmation."); err != nil {
			return fmt.Errorf("write rule-apply bulk note: %w", err)
		}
	}
	return nil
}
