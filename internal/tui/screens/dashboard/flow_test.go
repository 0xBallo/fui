package dashboard

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/theme"
)

type flowStub struct{ res reporting.CashFlowResult }

func (f flowStub) CashFlow(context.Context, reporting.CashFlowParams) (reporting.CashFlowResult, error) {
	return f.res, nil
}

func eur(t *testing.T, minor int64) domain.Money {
	t.Helper()
	c, err := domain.ParseCurrency("EUR")
	if err != nil {
		t.Fatal(err)
	}
	m, err := domain.NewMoney(minor, c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFlowItems(t *testing.T) {
	th := theme.Default()
	tests := []struct {
		name         string
		in, out, net int64
		want         []components.BarItem
	}{
		{"positive savings", 1000_00, -400_00, 600_00, []components.BarItem{
			{Label: "Income", Value: 1000_00, DisplayValue: "1,000.00", Style: th.Positive},
			{Label: "Expenses", Value: 400_00, DisplayValue: "-400.00", Style: th.Negative},
			{Label: "Savings", Value: 600_00, DisplayValue: "600.00", Style: th.Base},
		}},
		{"negative savings", 100_00, -250_00, -150_00, []components.BarItem{
			{Label: "Income", Value: 100_00, DisplayValue: "100.00", Style: th.Positive},
			{Label: "Expenses", Value: 250_00, DisplayValue: "-250.00", Style: th.Negative},
			{Label: "Savings", Value: 150_00, DisplayValue: "-150.00", Style: th.Base},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &flowBlock{loc: domain.LocaleENUS, th: th, res: reporting.CashFlowResult{
				TotalInflow: eur(t, tt.in), TotalOutflow: eur(t, tt.out), TotalNet: eur(t, tt.net),
			}}
			if diff := cmp.Diff(tt.want, s.items(), cmp.Comparer(func(a, b components.BarItem) bool {
				return a.Label == b.Label && a.Value == b.Value && a.DisplayValue == b.DisplayValue &&
					a.Style.String() == b.Style.String()
			})); diff != "" {
				t.Errorf("items mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFlowDisabledHidden(t *testing.T) {
	m := New(nil, domain.LocaleENUS, theme.Default(), 1)
	if got := m.sections[2].view(80); got != "" {
		t.Errorf("disabled flow view = %q, want empty", got)
	}
	m.SetFlowUseCases(flowStub{})
	s := m.sections[2]
	if cmd := s.load(sectionParams{}); cmd == nil {
		t.Fatal("enabled flow load = nil cmd")
	} else if _, ok := s.update(cmd()); !ok {
		t.Fatal("flow msg not consumed")
	}
	if got := s.view(80); !strings.Contains(got, "Cash flow") || !strings.Contains(got, "Savings") {
		t.Errorf("enabled flow view = %q", got)
	}
}
