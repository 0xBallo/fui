package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/domain"
)

func flagCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "x"}
	reportFilterFlags(cmd)
	reportGroupFlags(cmd)
	reportComparePeriodFlags(cmd)
	cmd.Flags().String("period", "", "")
	cmd.Flags().String("loan", "", "")
	cmd.Flags().Int("limit", 10, "")
	cmd.Flags().String("min-balance", "", "")
	cmd.Flags().String("horizon", "", "")
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestReadReportFilter(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, cmd *cobra.Command)
	}{
		{"empty", nil, false, func(t *testing.T, cmd *cobra.Command) {
			if reportFilterChanged(cmd) {
				t.Fatal("no filter flag was set")
			}
		}},
		{
			"values",
			[]string{"--category", "Food", "--category", "Fun", "--payee", "Coop", "--expense", "--min", "5"},
			false,
			func(t *testing.T, cmd *cobra.Command) {
				f, err := readReportFilter(cmd)
				if err != nil {
					t.Fatal(err)
				}
				if len(f.Categories) != 2 || f.Payees[0] != "Coop" || f.Sign != domain.TxSignExpense || f.MinAmount != "5" {
					t.Fatalf("filter = %+v", f)
				}
			},
		},
		{"income and expense", []string{"--income", "--expense"}, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := flagCmd(t, tc.args...)
			if tc.wantErr {
				if _, err := readReportFilter(cmd); err == nil {
					t.Fatal("want error")
				}
				return
			}
			tc.check(t, cmd)
		})
	}
}

func TestReportGrouping(t *testing.T) {
	tests := []struct {
		name, kind string
		args       []string
		group      domain.ReportGroup
		gran       domain.Granularity
		wantErr    bool
	}{
		{"none", "spending", nil, "", "", false},
		{"group by", "spending", []string{"--group-by", "Payee"}, domain.GroupPayee, "", false},
		{"balance granularity", "balance", []string{"--granularity", "week"}, "", domain.GranWeek, false},
		{"cash-flow granularity is the grouping", "cash-flow", []string{"--granularity", "year"}, domain.GroupYear, domain.GranYear, false},
		{"explicit group wins", "cash-flow", []string{"--group-by", "tag", "--granularity", "week"}, domain.GroupTag, domain.GranWeek, false},
		{"cash-flow day", "cash-flow", []string{"--granularity", "day"}, "", "", true},
		{"bad group", "spending", []string{"--group-by", "color"}, "", "", true},
		{"bad granularity", "balance", []string{"--granularity", "hour"}, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			group, gran, err := reportGrouping(flagCmd(t, tc.args...), tc.kind)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if group != tc.group || gran != tc.gran {
				t.Fatalf("got %q/%q, want %q/%q", group, gran, tc.group, tc.gran)
			}
		})
	}
}

func TestSavedSpecFromFlags(t *testing.T) {
	tests := []struct {
		name, kind string
		args       []string
		check      func(t *testing.T, s reporting.SavedSpec)
	}{
		{"lone account stays v1", reporting.KindBalance, []string{"--account", "Checking"}, func(t *testing.T, s reporting.SavedSpec) {
			if s.Account != "Checking" || s.Filter != nil {
				t.Fatalf("spec = %+v", s)
			}
		}},
		{"filter and grouping", reporting.KindSpending, []string{"--account", "A", "--category", "Food", "--group-by", "payee"}, func(t *testing.T, s reporting.SavedSpec) {
			if s.Account != "" || s.Filter == nil || s.Filter.Accounts[0] != "A" || s.GroupBy != "payee" {
				t.Fatalf("spec = %+v", s)
			}
			if s.Filter.From != "" || s.Filter.Month != "" {
				t.Fatalf("saved filter holds an absolute period: %+v", s.Filter)
			}
		}},
		{"compare", reporting.KindCompare, []string{"--a", "2026-09", "--b", "2026-Q4"}, func(t *testing.T, s reporting.SavedSpec) {
			if s.Compare == nil || s.Compare.A != "2026-09" || s.Compare.B != "2026-Q4" {
				t.Fatalf("spec = %+v", s)
			}
		}},
		{"forecast", reporting.KindForecast, []string{"--horizon", "12m", "--granularity", "week"}, func(t *testing.T, s reporting.SavedSpec) {
			if s.Horizon != "12m" || s.Granularity != "week" || s.Compare != nil {
				t.Fatalf("spec = %+v", s)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := savedSpecFromFlags(flagCmd(t, tc.args...), tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, s)
		})
	}
}
