package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestApplyRulePriorityAndFieldPrecedence(t *testing.T) {
	rules := []Rule{
		{
			Name:     "set-category-first",
			Priority: 1,
			Enabled:  true,
			Actions: RuleActions{
				CategoryID: 10,
			},
		},
		{
			Name:     "set-category-and-payee-later",
			Priority: 2,
			Enabled:  true,
			Actions: RuleActions{
				CategoryID: 20,
				PayeeID:    30,
			},
		},
		{
			Name:     "set-tags-last",
			Priority: 3,
			Enabled:  true,
			Actions: RuleActions{
				TagIDs: []TagID{40, 50, 40},
			},
		},
	}

	got, err := Apply(rules, testRuleTransaction(t, 7, -123_45), "any text")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got.CategoryID != 10 || got.CategoryRule != "set-category-first" {
		t.Fatalf("category = (%d,%q), want (10,%q)", got.CategoryID, got.CategoryRule, "set-category-first")
	}
	if got.PayeeID != 30 || got.PayeeRule != "set-category-and-payee-later" {
		t.Fatalf("payee = (%d,%q), want (30,%q)", got.PayeeID, got.PayeeRule, "set-category-and-payee-later")
	}
	if got.TagsRule != "set-tags-last" {
		t.Fatalf("tags rule = %q, want %q", got.TagsRule, "set-tags-last")
	}
	if len(got.TagIDs) != 2 || got.TagIDs[0] != 40 || got.TagIDs[1] != 50 {
		t.Fatalf("tag IDs = %v, want [40 50]", got.TagIDs)
	}
}

func TestApplySortsByPriorityStable(t *testing.T) {
	rules := []Rule{
		{
			Name:     "first",
			Priority: 10,
			Enabled:  true,
			Actions: RuleActions{
				CategoryID: 1,
			},
		},
		{
			Name:     "second-same-priority",
			Priority: 10,
			Enabled:  true,
			Actions: RuleActions{
				CategoryID: 2,
			},
		},
	}

	got, err := Apply(rules, testRuleTransaction(t, 1, -100), "")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.CategoryID != 1 || got.CategoryRule != "first" {
		t.Fatalf("category winner = (%d,%q), want (1,%q)", got.CategoryID, got.CategoryRule, "first")
	}
}

func TestApplyMatchesConditions(t *testing.T) {
	minorMin := int64(10_00)
	minorMax := int64(20_00)
	tx := testRuleTransaction(t, 5, -15_00)
	rules := []Rule{
		{
			Name:     "contains+amount+account",
			Priority: 1,
			Enabled:  true,
			Conditions: RuleConditions{
				DescriptionContains: "amazon",
				Amount: RuleAmountRange{
					MinAbsMinor: &minorMin,
					MaxAbsMinor: &minorMax,
					Sign:        RuleSignOutflow,
				},
				AccountIDs: []AccountID{5, 5},
			},
			Actions: RuleActions{
				CategoryID: 11,
				PayeeID:    12,
				TagIDs:     []TagID{13},
			},
		},
	}

	got, err := Apply(rules, tx, "  AMAZOn marketplace  ")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.CategoryID != 11 || got.PayeeID != 12 || len(got.TagIDs) != 1 || got.TagIDs[0] != 13 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestApplyNoMatchReturnsEmpty(t *testing.T) {
	rules := []Rule{
		{
			Name:     "equals-only",
			Priority: 1,
			Enabled:  true,
			Conditions: RuleConditions{
				DescriptionEquals: "wire transfer",
			},
			Actions: RuleActions{CategoryID: 1},
		},
	}
	got, err := Apply(rules, testRuleTransaction(t, 1, 100), "salary")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.CategoryRule != "" || got.PayeeRule != "" || got.TagsRule != "" {
		t.Fatalf("result should be empty, got %+v", got)
	}
}

func TestRuleJSONStrictDecoding(t *testing.T) {
	valid := `{"name":"r1","priority":1,"enabled":true,"conditions":{"description_contains":"amazon","amount":{"sign":"outflow"},"account_ids":[1]},"actions":{"category_id":2,"tag_ids":[3,3]}}`
	var got Rule
	if err := json.Unmarshal([]byte(valid), &got); err != nil {
		t.Fatalf("valid rule JSON: %v", err)
	}
	if got.Conditions.DescriptionContains != "amazon" {
		t.Fatalf("normalized contains = %q, want %q", got.Conditions.DescriptionContains, "amazon")
	}
	if len(got.Actions.TagIDs) != 1 || got.Actions.TagIDs[0] != 3 {
		t.Fatalf("tag IDs = %v, want [3]", got.Actions.TagIDs)
	}
	if got.Conditions.Amount.Sign != RuleSignOutflow {
		t.Fatalf("sign = %q, want %q", got.Conditions.Amount.Sign, RuleSignOutflow)
	}

	cases := []string{
		`{"name":"r1","priority":1,"enabled":true,"conditions":{},"actions":{"category_id":2},"extra":1}`,
		`{"name":"r1","priority":1,"enabled":true,"conditions":{"amount":{"sign":"outflow","unknown":1}},"actions":{"category_id":2}}`,
		`{"name":"r1","priority":1,"enabled":true,"conditions":{},"actions":{"category_id":2,"unknown":1}}`,
		`{"name":"r1","priority":1,"enabled":true,"conditions":{"amount":{"sign":"regex"}},"actions":{"category_id":2}}`,
	}
	for _, input := range cases {
		var r Rule
		if err := json.Unmarshal([]byte(input), &r); err == nil {
			t.Fatalf("json.Unmarshal(%s) unexpectedly succeeded", input)
		}
	}
}

func TestRuleValidationErrors(t *testing.T) {
	minorMin := int64(20)
	minorMax := int64(10)
	_, err := NewRule(Rule{
		Name:    "invalid-range",
		Enabled: true,
		Conditions: RuleConditions{
			Amount: RuleAmountRange{
				MinAbsMinor: &minorMin,
				MaxAbsMinor: &minorMax,
			},
		},
		Actions: RuleActions{CategoryID: 1},
	})
	if err == nil {
		t.Fatal("NewRule should fail on invalid range")
	}
	var de *Error
	if !errors.As(err, &de) || de.Code != CodeInvalidValue {
		t.Fatalf("error = %v, want domain invalid_value", err)
	}

	_, err = NewRule(Rule{
		Name:    "no-actions",
		Enabled: true,
		Actions: RuleActions{},
	})
	if err == nil {
		t.Fatal("NewRule should fail on missing actions")
	}
	if !errors.As(err, &de) || de.Code != CodeMissingField {
		t.Fatalf("error = %v, want domain missing_field", err)
	}
}

func testRuleTransaction(t *testing.T, accountID AccountID, minor int64) Transaction {
	t.Helper()
	m, err := NewMoney(minor, cur(t, "EUR"))
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	d, err := NewDate(2026, 1, 2)
	if err != nil {
		t.Fatalf("NewDate: %v", err)
	}
	tx, err := NewTransaction(Transaction{
		AccountID:   accountID,
		Date:        d,
		Amount:      m,
		Description: "x",
		Status:      StatusBooked,
		Kind:        KindOther,
	})
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}
	return tx
}
