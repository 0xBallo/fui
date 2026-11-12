package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

// Rule applies categorization actions to matching transactions.
type Rule struct {
	Name       string         `json:"name"`
	Priority   int            `json:"priority"`
	Enabled    bool           `json:"enabled"`
	Conditions RuleConditions `json:"conditions"`
	Actions    RuleActions    `json:"actions"`
}

// RuleConditions are all matched in AND.
type RuleConditions struct {
	DescriptionContains string          `json:"description_contains,omitempty"`
	DescriptionEquals   string          `json:"description_equals,omitempty"`
	Amount              RuleAmountRange `json:"amount,omitempty"`
	AccountIDs          []AccountID     `json:"account_ids,omitempty"`
}

// RuleAmountSign filters on transaction sign.
type RuleAmountSign string

// Amount sign filters.
const (
	RuleSignAny     RuleAmountSign = "any"
	RuleSignInflow  RuleAmountSign = "inflow"
	RuleSignOutflow RuleAmountSign = "outflow"
)

// Valid reports whether s is a known sign filter.
func (s RuleAmountSign) Valid() bool {
	switch s {
	case "", RuleSignAny, RuleSignInflow, RuleSignOutflow:
		return true
	}
	return false
}

// RuleAmountRange filters on absolute amount and sign.
type RuleAmountRange struct {
	MinAbsMinor *int64         `json:"min_abs_minor,omitempty"`
	MaxAbsMinor *int64         `json:"max_abs_minor,omitempty"`
	Sign        RuleAmountSign `json:"sign,omitempty"`
}

// RuleActions define the values a matching rule can set.
type RuleActions struct {
	CategoryID CategoryID `json:"category_id,omitempty"`
	PayeeID    PayeeID    `json:"payee_id,omitempty"`
	TagIDs     []TagID    `json:"tag_ids,omitempty"`
}

// RuleApplyResult contains the first winning value and rule name for each field.
type RuleApplyResult struct {
	CategoryID   CategoryID
	CategoryRule string
	PayeeID      PayeeID
	PayeeRule    string
	TagIDs       []TagID
	TagsRule     string
}

// NewRule validates r and returns a normalized copy.
func NewRule(r Rule) (Rule, error) {
	name, err := requireName("name", r.Name)
	if err != nil {
		return Rule{}, err
	}
	r.Name = name

	conds, err := normalizeConditions(r.Conditions)
	if err != nil {
		return Rule{}, err
	}
	r.Conditions = conds

	actions, err := normalizeActions(r.Actions)
	if err != nil {
		return Rule{}, err
	}
	r.Actions = actions

	return r, nil
}

// Apply runs enabled rules by ascending priority. The first rule that sets a
// specific field wins for that field; lower-priority rules can still set other
// fields that are still unset.
func Apply(rules []Rule, tx Transaction, normalizedText string) (RuleApplyResult, error) {
	if tx.AccountID == 0 {
		return RuleApplyResult{}, missingField("account_id")
	}
	if tx.Amount.Currency().IsZero() {
		return RuleApplyResult{}, missingField("amount")
	}

	normalizedText = normalizeRuleText(normalizedText)
	sorted, err := sortedEnabledRules(rules)
	if err != nil {
		return RuleApplyResult{}, err
	}

	var res RuleApplyResult
	for _, rule := range sorted {
		ok, err := rule.Conditions.match(tx, normalizedText)
		if err != nil {
			return RuleApplyResult{}, err
		}
		if !ok {
			continue
		}
		res.assign(rule)
		if res.CategoryRule != "" && res.PayeeRule != "" && res.TagsRule != "" {
			break
		}
	}
	return res, nil
}

// UnmarshalJSON decodes and validates Rule with unknown fields rejected.
func (r *Rule) UnmarshalJSON(data []byte) error {
	type rawRule Rule
	var raw rawRule
	if err := strictUnmarshalJSON(data, &raw); err != nil {
		return err
	}
	norm, err := NewRule(Rule(raw))
	if err != nil {
		return err
	}
	*r = norm
	return nil
}

// UnmarshalJSON decodes and validates RuleConditions with unknown fields rejected.
func (c *RuleConditions) UnmarshalJSON(data []byte) error {
	type rawCond RuleConditions
	var raw rawCond
	if err := strictUnmarshalJSON(data, &raw); err != nil {
		return err
	}
	norm, err := normalizeConditions(RuleConditions(raw))
	if err != nil {
		return err
	}
	*c = norm
	return nil
}

// UnmarshalJSON decodes and validates RuleAmountRange with unknown fields rejected.
func (a *RuleAmountRange) UnmarshalJSON(data []byte) error {
	type rawAmount RuleAmountRange
	var raw rawAmount
	if err := strictUnmarshalJSON(data, &raw); err != nil {
		return err
	}
	norm, err := normalizeAmountRange(RuleAmountRange(raw))
	if err != nil {
		return err
	}
	*a = norm
	return nil
}

// UnmarshalJSON decodes and validates RuleActions with unknown fields rejected.
func (a *RuleActions) UnmarshalJSON(data []byte) error {
	type rawAction RuleActions
	var raw rawAction
	if err := strictUnmarshalJSON(data, &raw); err != nil {
		return err
	}
	norm, err := normalizeActions(RuleActions(raw))
	if err != nil {
		return err
	}
	*a = norm
	return nil
}

func normalizeConditions(c RuleConditions) (RuleConditions, error) {
	c.DescriptionContains = normalizeRuleText(c.DescriptionContains)
	c.DescriptionEquals = normalizeRuleText(c.DescriptionEquals)

	amount, err := normalizeAmountRange(c.Amount)
	if err != nil {
		return RuleConditions{}, err
	}
	c.Amount = amount

	seen := make(map[AccountID]struct{}, len(c.AccountIDs))
	ids := make([]AccountID, 0, len(c.AccountIDs))
	for _, id := range c.AccountIDs {
		if id <= 0 {
			return RuleConditions{}, invalidValue("account_ids", "must be positive")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	c.AccountIDs = ids
	return c, nil
}

func normalizeAmountRange(r RuleAmountRange) (RuleAmountRange, error) {
	if !r.Sign.Valid() {
		return RuleAmountRange{}, invalidValue("sign", string(r.Sign))
	}
	if r.Sign == "" {
		r.Sign = RuleSignAny
	}
	if r.MinAbsMinor != nil && *r.MinAbsMinor < 0 {
		return RuleAmountRange{}, invalidValue("min_abs_minor", "must be >= 0")
	}
	if r.MaxAbsMinor != nil && *r.MaxAbsMinor < 0 {
		return RuleAmountRange{}, invalidValue("max_abs_minor", "must be >= 0")
	}
	if r.MinAbsMinor != nil && r.MaxAbsMinor != nil && *r.MinAbsMinor > *r.MaxAbsMinor {
		return RuleAmountRange{}, newError(ErrInvalidValue,
			map[string]any{"field": "amount", "min_abs_minor": *r.MinAbsMinor, "max_abs_minor": *r.MaxAbsMinor},
			"invalid amount range: min_abs_minor > max_abs_minor")
	}
	return r, nil
}

func normalizeActions(a RuleActions) (RuleActions, error) {
	if a.CategoryID < 0 {
		return RuleActions{}, invalidValue("category_id", "must be positive")
	}
	if a.PayeeID < 0 {
		return RuleActions{}, invalidValue("payee_id", "must be positive")
	}
	seen := make(map[TagID]struct{}, len(a.TagIDs))
	tags := make([]TagID, 0, len(a.TagIDs))
	for _, id := range a.TagIDs {
		if id <= 0 {
			return RuleActions{}, invalidValue("tag_ids", "must be positive")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		tags = append(tags, id)
	}
	a.TagIDs = tags
	if a.CategoryID == 0 && a.PayeeID == 0 && len(a.TagIDs) == 0 {
		return RuleActions{}, newError(ErrMissingField, map[string]any{"field": "actions"},
			"missing actions: at least one of category_id, payee_id or tag_ids is required")
	}
	return a, nil
}

func (c RuleConditions) match(tx Transaction, normalizedText string) (bool, error) {
	if c.DescriptionContains != "" && !strings.Contains(normalizedText, c.DescriptionContains) {
		return false, nil
	}
	if c.DescriptionEquals != "" && normalizedText != c.DescriptionEquals {
		return false, nil
	}

	ok, err := c.Amount.match(tx.Amount)
	if err != nil || !ok {
		return ok, err
	}

	if len(c.AccountIDs) > 0 && !slices.Contains(c.AccountIDs, tx.AccountID) {
		return false, nil
	}
	return true, nil
}

func (a RuleAmountRange) match(amount Money) (bool, error) {
	if amount.Currency().IsZero() {
		return false, missingField("amount")
	}
	switch a.Sign {
	case RuleSignAny:
		// no sign filter
	case RuleSignInflow:
		if amount.Sign() <= 0 {
			return false, nil
		}
	case RuleSignOutflow:
		if amount.Sign() >= 0 {
			return false, nil
		}
	}

	abs := amount.Minor()
	if abs < 0 {
		abs = -abs
	}
	if a.MinAbsMinor != nil && abs < *a.MinAbsMinor {
		return false, nil
	}
	if a.MaxAbsMinor != nil && abs > *a.MaxAbsMinor {
		return false, nil
	}
	return true, nil
}

func normalizeRuleText(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func sortedEnabledRules(rules []Rule) ([]Rule, error) {
	sorted := make([]Rule, 0, len(rules))
	for i := range rules {
		r, err := NewRule(rules[i])
		if err != nil {
			return nil, err
		}
		if !r.Enabled {
			continue
		}
		sorted = append(sorted, r)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted, nil
}

func (r *RuleApplyResult) assign(rule Rule) {
	if r.CategoryRule == "" && rule.Actions.CategoryID != 0 {
		r.CategoryID = rule.Actions.CategoryID
		r.CategoryRule = rule.Name
	}
	if r.PayeeRule == "" && rule.Actions.PayeeID != 0 {
		r.PayeeID = rule.Actions.PayeeID
		r.PayeeRule = rule.Name
	}
	if r.TagsRule == "" && len(rule.Actions.TagIDs) > 0 {
		r.TagIDs = slices.Clone(rule.Actions.TagIDs)
		r.TagsRule = rule.Name
	}
}

func strictUnmarshalJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		if err == nil {
			return invalidValue("json", "multiple JSON values")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}
