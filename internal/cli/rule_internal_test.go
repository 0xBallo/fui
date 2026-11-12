package cli

import (
	"errors"
	"testing"

	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
)

func TestRequireRulesNil(t *testing.T) {
	if err := requireRules(&Deps{}); err == nil {
		t.Fatal("requireRules(nil) expected error")
	}
}

func TestParseRuleAmountNormalizesAbsolute(t *testing.T) {
	deps := &Deps{Config: &config.Config{Currency: "EUR", Locale: config.LocaleIT}}
	got, err := parseRuleAmount(deps, "-10,50")
	if err != nil {
		t.Fatalf("parseRuleAmount: %v", err)
	}
	if got == nil || *got != 1050 {
		t.Fatalf("parseRuleAmount = %v, want 1050", got)
	}
}

func TestRuleEditRequestEnableDisableConflict(t *testing.T) {
	cmd := newRuleEditCmd(&Deps{})
	if err := cmd.Flags().Set("enable", "true"); err != nil {
		t.Fatalf("set enable: %v", err)
	}
	if err := cmd.Flags().Set("disable", "true"); err != nil {
		t.Fatalf("set disable: %v", err)
	}
	_, err := ruleEditRequest(cmd, &Deps{}, "r")
	if err == nil {
		t.Fatal("ruleEditRequest expected conflict error")
	}
	var uerr *usageError
	if !errors.As(err, &uerr) {
		t.Fatalf("ruleEditRequest err = %T, want usageError", err)
	}
}

func TestRuleTestRequestTxConflictsWithText(t *testing.T) {
	cmd := newRuleTestCmd(&Deps{})
	if err := cmd.Flags().Set("tx", "10"); err != nil {
		t.Fatalf("set tx: %v", err)
	}
	_, err := ruleTestRequest(cmd, &Deps{}, []string{"text"})
	if err == nil {
		t.Fatal("ruleTestRequest expected conflict error")
	}
}

func TestRuleTestRequestRequiresTextWhenNoTx(t *testing.T) {
	cmd := newRuleTestCmd(&Deps{})
	_, err := ruleTestRequest(cmd, &Deps{}, nil)
	if err == nil {
		t.Fatal("ruleTestRequest expected arg-count error")
	}
}

func TestCurrencyOfAccountNotFound(t *testing.T) {
	h := newStandalone(t, domain.LocaleENUS, config.LocaleUS)
	h.seedAccount(t, "Checking")
	_, err := currencyOfAccount(t.Context(), h.deps, "Missing")
	if err == nil {
		t.Fatal("currencyOfAccount expected not_found")
	}
}
