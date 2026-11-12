package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func TestNormalizeSavedKind(t *testing.T) {
	tests := map[string]string{
		"balance":          reporting.KindBalance,
		"budget-vs-actual": reporting.KindBudget,
		"remaining-debt":   reporting.KindDebt,
		"interest-paid":    reporting.KindInterest,
		"networth":         reporting.KindNetWorth,
		"top-payees":       reporting.KindPayees,
		"forecast":         reporting.KindForecast,
		"spending":         reporting.KindSpending,
		"cash-flow":        reporting.KindCashFlow,
		"custom":           "custom",
	}
	for in, want := range tests {
		if got := normalizeSavedKind(in); got != want {
			t.Fatalf("normalizeSavedKind(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReportPeriodValue(t *testing.T) {
	cmd := &cobra.Command{Use: "report"}
	reportKindPeriodFlag(cmd)

	t.Run("month", func(t *testing.T) {
		mustSetFlag(t, cmd, "period", "2026-11")
		p, err := reportPeriodValue(cmd, 1)
		if err != nil {
			t.Fatalf("reportPeriodValue month: %v", err)
		}
		if p.From().String() != "2026-11-01" || p.To().String() != "2026-11-30" {
			t.Fatalf("period = %s..%s, want 2026-11-01..2026-11-30", p.From(), p.To())
		}
	})

	t.Run("range", func(t *testing.T) {
		mustSetFlag(t, cmd, "period", "2026-11-05..2026-11-20")
		p, err := reportPeriodValue(cmd, 1)
		if err != nil {
			t.Fatalf("reportPeriodValue range: %v", err)
		}
		if p.From().String() != "2026-11-05" || p.To().String() != "2026-11-20" {
			t.Fatalf("period = %s..%s, want 2026-11-05..2026-11-20", p.From(), p.To())
		}
	})

	t.Run("relative unsupported", func(t *testing.T) {
		mustSetFlag(t, cmd, "period", "this-month")
		if _, err := reportPeriodValue(cmd, 1); err == nil {
			t.Fatal("relative period accepted, want invalid_value")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		mustSetFlag(t, cmd, "period", "bad-period")
		if _, err := reportPeriodValue(cmd, 1); err == nil {
			t.Fatal("invalid period accepted")
		}
	})
}

func TestMissingField(t *testing.T) {
	err := missingField("loan")
	var derr *domain.Error
	if !errors.As(err, &derr) {
		t.Fatalf("missingField returns %T, want *domain.Error", err)
	}
	if derr.Code != domain.CodeMissingField {
		t.Fatalf("missingField code = %q, want %q", derr.Code, domain.CodeMissingField)
	}
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set flag %s=%q: %v", name, value, err)
	}
}
