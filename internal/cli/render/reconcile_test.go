package render

import (
	"strings"
	"testing"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

func TestWriteReconcileTickMode(t *testing.T) {
	eur, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := domain.NewMoney(9000, eur)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := domain.NewMoney(-2000, eur)
	if err != nil {
		t.Fatal(err)
	}
	plan := app.ReconcilePlan{
		Account:          domain.Account{Name: "Checking"},
		StatementBalance: stmt,
		Difference:       diff,
		Ticking:          true,
		Ticked:           []domain.TransactionID{1, 2},
	}
	tests := []struct {
		name   string
		format Format
		want   []string
		absent []string
	}{
		{"table", Table, []string{"Ticked transactions: 2", "Unreconciled transactions: 0"}, []string{"pass --yes"}},
		{"json", JSON, []string{`"ticked":2`, `"difference":-2000`}, nil},
		{"csv", CSV, nil, []string{"Ticked"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			if err := WriteReconcile(&b, tt.format, domain.LocaleENUS, false, plan, nil); err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(b.String(), w) {
					t.Errorf("output missing %q:\n%s", w, b.String())
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(b.String(), a) {
					t.Errorf("output has %q:\n%s", a, b.String())
				}
			}
		})
	}

	plan.Ticking, plan.Ticked = false, nil
	var b strings.Builder
	if err := WriteReconcile(&b, JSON, domain.LocaleENUS, false, plan, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), `"ticked"`) {
		t.Errorf("non-tick output has ticked: %s", b.String())
	}
}
