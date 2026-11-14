package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

// NewReportCompareCmd builds `fui report compare`, the standalone form of `fui report run compare`.
func NewReportCompareCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "compare",
		Short:   "Compare the spending of two periods",
		Example: "  fui report compare --a last-month --b this-month --by category --json",
		Args:    noArgs(),
		RunE:    func(cmd *cobra.Command, _ []string) error { return RunReportCompare(cmd, deps) },
	}
	ReportCompareFlags(cmd)
	return cmd
}

// ReportCompareFlags registers the --a, --b and --by flags on cmd, for `fui report run`.
func ReportCompareFlags(cmd *cobra.Command) {
	cmd.Flags().String("a", "", "first period, e.g. last-month (default last-month)")
	cmd.Flags().String("b", "", "second period, e.g. this-month (default this-month)")
	cmd.Flags().String("by", "", "break the comparison down by category")
}

// RunReportCompare runs the compare report from the flags of cmd.
func RunReportCompare(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	analytics, err := requireAnalytics(deps)
	if err != nil {
		return err
	}
	a, _ := cmd.Flags().GetString("a")
	b, _ := cmd.Flags().GetString("b")
	by, _ := cmd.Flags().GetString("by")
	if by != "" && by != "category" {
		return &domain.Error{
			Code:    domain.CodeInvalidValue,
			Message: fmt.Sprintf("unknown --by value %q (allowed: category)", by),
			Details: map[string]any{"field": "by", "value": by},
		}
	}
	out, err := analytics.ComparePeriods(cmd.Context(), reporting.CompareRequest{A: a, B: b, ByCategory: by == "category"})
	if err != nil {
		return err
	}
	return writeCompare(deps, rt, out)
}

func writeCompare(deps *Deps, rt runtimeFlags, out reporting.CompareReport) error {
	if rt.format == render.JSON {
		return render.WriteJSON(deps.Stdout, out)
	}
	rows := render.Rows{Header: []string{"CATEGORY", "A", "B", "DELTA", "DELTA %"}, MoneyCols: []int{1, 2, 3}}
	all := append(append([]reporting.CompareRowView{}, out.Rows...), out.Total)
	for i, r := range all {
		name := r.Category
		if i == len(all)-1 {
			name = "TOTAL"
		}
		cells := []string{name}
		for _, amt := range []reporting.Amount{r.A, r.B, r.Delta} {
			s, err := formatAmount(amt, rt.format, deps.Locale())
			if err != nil {
				return err
			}
			cells = append(cells, s)
		}
		cells = append(cells, permilleText(r.DeltaPermille))
		rows.Body = append(rows.Body, cells)
	}
	if rt.format == render.CSV {
		return render.WriteCSV(deps.Stdout, rows)
	}
	return render.WriteTable(deps.Stdout, rows, rt.color)
}

// permilleText renders a permille change as a percentage with one decimal; "-" when undefined.
func permilleText(p *int64) string {
	if p == nil {
		return "-"
	}
	n := *p
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	return sign + strconv.FormatInt(n/10, 10) + "." + strconv.FormatInt(n%10, 10) + "%"
}
