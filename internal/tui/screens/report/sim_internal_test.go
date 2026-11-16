package report

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
)

func TestParseSimRates(t *testing.T) {
	tests := []struct {
		in      string
		want    simulating.Rates
		wantErr bool
	}{
		{in: "5", want: simulating.Rates{PessimisticBP: 300, BaseBP: 500, OptimisticBP: 700}},
		{in: "3, 4.5 ,6.25%", want: simulating.Rates{PessimisticBP: 300, BaseBP: 450, OptimisticBP: 625}},
		{in: "1,2", wantErr: true},
		{in: "x", wantErr: true},
		{in: "", wantErr: true},
		{in: "1.234", wantErr: true},
	}
	for _, tc := range tests {
		got, err := parseSimRates(tc.in)
		if (err != nil) != tc.wantErr {
			t.Fatalf("parseSimRates(%q) err = %v", tc.in, err)
		}
		if err == nil {
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("parseSimRates(%q) (-want +got):\n%s", tc.in, diff)
			}
		}
	}
}

func TestBuildSimRequest(t *testing.T) {
	eur, _ := domain.ParseCurrency("EUR")
	v := defaultSimValues()
	v["initial"], v["positions"], v["tax"] = "1,500.50", "yes", "26"
	v["frequency"], v["growth"] = "quarterly", "2"
	req, err := buildSimRequest(simulating.KindPAC, v, domain.LocaleENUS, eur)
	if err != nil {
		t.Fatal(err)
	}
	want := simulating.PACParams{
		InitialMinor: 150050, InitialCurrentPositions: true, ContributionMinor: 20000, Frequency: "quarterly",
		ContributionGrowthBP: 200, Years: 10, Compounding: "monthly", TaxBP: 2600,
		Rates: simulating.Rates{PessimisticBP: 300, BaseBP: 500, OptimisticBP: 700},
	}
	if diff := cmp.Diff(want, *req.PAC); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}

	cf, err := buildSimRequest(simulating.KindCashFlow, v, domain.LocaleENUS, eur)
	if err != nil || cf.CashFlow.Months != 12 {
		t.Fatalf("cashflow = %+v, %v", cf, err)
	}

	for field, bad := range map[string]string{"years": "0", "initial": "abc", "positions": "maybe", "cost": "1.234", "contribution": "-x"} {
		w := defaultSimValues()
		w[field] = bad
		if _, err := buildSimRequest(simulating.KindPAC, w, domain.LocaleENUS, eur); err == nil {
			t.Fatalf("%s=%q should fail", field, bad)
		}
	}
}

func TestSparkline(t *testing.T) {
	if got := sparkline([]int64{0, 50, 100}); got != "▁▄█" {
		t.Fatalf("sparkline = %q", got)
	}
	if got := sparkline([]int64{7, 7}); got != "▁▁" {
		t.Fatalf("flat sparkline = %q", got)
	}
	if got := sparkline(nil); got != "" {
		t.Fatalf("empty sparkline = %q", got)
	}
}
