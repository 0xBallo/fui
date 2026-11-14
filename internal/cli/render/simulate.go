package render

import (
	"fmt"
	"io"
	"strconv"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/domain/simulate"
)

// SimulationRun is the --json shape of one simulation run: the request that was computed and
// its result.
type SimulationRun struct {
	Request simulating.RunRequest `json:"request"`
	Result  simulating.RunResult  `json:"result"`
}

type simMoney struct {
	cur domain.Currency
	loc domain.Locale
	csv bool
	err error
}

func (m *simMoney) fmt(minor int64) string {
	v, err := domain.NewMoney(minor, m.cur)
	if err != nil {
		m.err = fmt.Errorf("format simulation amount %d: %w", minor, err)
		return ""
	}
	if m.csv {
		return FormatMoneyMachine(v)
	}
	return FormatMoney(v, m.loc)
}

// FormatBasisPoints renders basis points as a percentage, e.g. 450 as "4.50%".
func FormatBasisPoints(bp int64) string {
	sign := ""
	if bp < 0 {
		sign, bp = "-", -bp
	}
	return fmt.Sprintf("%s%d.%02d%%", sign, bp/100, bp%100)
}

// WriteSimulationRun renders one simulation run. Investment and PAC runs show the three rates
// side by side; chart adds a text bar per row in table output.
func WriteSimulationRun(
	w io.Writer, f Format, loc domain.Locale, color bool, cur domain.Currency,
	req simulating.RunRequest, out simulating.RunResult, chart bool,
) error {
	if f == JSON {
		return WriteJSON(w, SimulationRun{Request: req, Result: out})
	}
	m := &simMoney{cur: cur, loc: loc, csv: f == CSV}
	switch {
	case out.Investment != nil:
		return writeTriple(w, f, color, m, ratesOf(req), *out.Investment, chart)
	case out.PAC != nil:
		return writeTriple(w, f, color, m, ratesOf(req), *out.PAC, chart)
	case out.CashFlow != nil:
		return writeCashFlow(w, f, color, m, *out.CashFlow, chart)
	}
	return fmt.Errorf("render simulation: empty %q result", out.Kind)
}

func ratesOf(req simulating.RunRequest) simulating.Rates {
	switch {
	case req.Investment != nil:
		return req.Investment.Rates
	case req.PAC != nil:
		return req.PAC.Rates
	}
	return simulating.Rates{}
}

func writeTriple(w io.Writer, f Format, color bool, m *simMoney, r simulating.Rates, t simulate.Triple, chart bool) error {
	results := [3]simulate.Result{t.Pessimistic, t.Base, t.Optimistic}
	years := rowsByYear(results)
	yearly := Rows{
		Header: []string{
			"YEAR", "CONTRIBUTED", "NOMINAL LOW", "NOMINAL BASE", "NOMINAL HIGH", "REAL BASE",
		},
		MoneyCols: []int{1, 2, 3, 4, 5},
	}
	base := make([]int64, 0, len(years))
	for _, y := range years {
		yearly.Body = append(yearly.Body, []string{
			strconv.Itoa(y[1].Year), m.fmt(y[1].Contributions), m.fmt(y[0].Nominal), m.fmt(y[1].Nominal),
			m.fmt(y[2].Nominal), m.fmt(y[1].Real),
		})
		base = append(base, y[1].Nominal)
	}
	if m.err != nil {
		return m.err
	}
	if f == CSV {
		return WriteCSV(w, yearly)
	}
	if chart {
		yearly.Header = append(yearly.Header, "CHART")
		for i, bar := range barStrings(base) {
			yearly.Body[i] = append(yearly.Body[i], bar)
		}
	}
	summary := tripleSummary(m, r, results)
	if m.err != nil {
		return m.err
	}
	if err := WriteTable(w, summary, color); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return fmt.Errorf("write simulation: %w", err)
	}
	return WriteTable(w, yearly, color)
}

func tripleSummary(m *simMoney, r simulating.Rates, results [3]simulate.Result) Rows {
	summary := Rows{
		Header: []string{
			"", "PESSIMISTIC " + FormatBasisPoints(r.PessimisticBP),
			"BASE " + FormatBasisPoints(r.BaseBP), "OPTIMISTIC " + FormatBasisPoints(r.OptimisticBP),
		},
		MoneyCols: []int{1, 2, 3},
	}
	metrics := []struct {
		name string
		get  func(simulate.Result) int64
	}{
		{"Contributed", func(r simulate.Result) int64 { return r.Contributions }},
		{"Nominal value", func(r simulate.Result) int64 { return r.Nominal }},
		{"Real value", func(r simulate.Result) int64 { return r.Real }},
		{"Gross gain", func(r simulate.Result) int64 { return r.GrossGain }},
		{"Tax", func(r simulate.Result) int64 { return r.Tax }},
		{"Net gain", func(r simulate.Result) int64 { return r.NetGain }},
	}
	for _, mt := range metrics {
		summary.Body = append(summary.Body, []string{
			mt.name, m.fmt(mt.get(results[0])), m.fmt(mt.get(results[1])), m.fmt(mt.get(results[2])),
		})
	}
	return summary
}

// rowsByYear zips the per-year rows of the three results; shorter series are padded with zeros.
func rowsByYear(results [3]simulate.Result) [][3]simulate.YearRow {
	n := 0
	for _, r := range results {
		n = max(n, len(r.Rows))
	}
	out := make([][3]simulate.YearRow, n)
	for i := range n {
		for j, r := range results {
			if i < len(r.Rows) {
				out[i][j] = r.Rows[i]
			}
		}
	}
	return out
}

func writeCashFlow(w io.Writer, f Format, color bool, m *simMoney, c simulating.CashFlowResult, chart bool) error {
	months := Rows{Header: []string{"MONTH END", "BASE", "SCENARIO", "DELTA"}, MoneyCols: []int{1, 2, 3}}
	scenario := make([]int64, 0, len(c.Months))
	for _, mo := range c.Months {
		months.Body = append(months.Body, []string{
			mo.End, m.fmt(mo.BaseMinor), m.fmt(mo.ScenarioMinor), m.fmt(mo.ScenarioMinor - mo.BaseMinor),
		})
		scenario = append(scenario, mo.ScenarioMinor)
	}
	if m.err != nil {
		return m.err
	}
	if f == CSV {
		return WriteCSV(w, months)
	}
	if chart {
		months.Header = append(months.Header, "CHART")
		for i, bar := range barStrings(scenario) {
			months.Body[i] = append(months.Body[i], bar)
		}
	}
	summary := Rows{Header: []string{"", "VALUE"}, MoneyCols: []int{1}}
	summary.Body = append(summary.Body,
		[]string{"Opening balance", m.fmt(c.OpeningMinor)},
		[]string{"First month below zero", orDash(c.FirstBelowZero)},
		[]string{"Difference at horizon", m.fmt(c.EndDeltaMinor)},
		[]string{"Runway (months)", runwayText(c.RunwayMonths)},
	)
	if m.err != nil {
		return m.err
	}
	if err := WriteTable(w, summary, color); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return fmt.Errorf("write simulation: %w", err)
	}
	return WriteTable(w, months, color)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func runwayText(months int) string {
	if months == 0 {
		return "-"
	}
	return strconv.Itoa(months)
}

type savedSimulationDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func savedSimulationOf(s app.SavedSimulation) savedSimulationDTO {
	return savedSimulationDTO{ID: int64(s.ID), Name: s.Name, Kind: s.Kind}
}

// WriteSimulationSaved confirms a saved simulation.
func WriteSimulationSaved(w io.Writer, f Format, s app.SavedSimulation) error {
	if f == JSON {
		return WriteJSON(w, savedSimulationOf(s))
	}
	if _, err := fmt.Fprintf(w, "Saved simulation %q (%s).\n", s.Name, s.Kind); err != nil {
		return fmt.Errorf("write saved simulation: %w", err)
	}
	return nil
}

// WriteSimulationRemoved confirms a removed saved simulation.
func WriteSimulationRemoved(w io.Writer, f Format, name string) error {
	if f == JSON {
		return WriteJSON(w, struct {
			Removed string `json:"removed"`
		}{name})
	}
	if _, err := fmt.Fprintf(w, "Removed simulation %q.\n", name); err != nil {
		return fmt.Errorf("write removed simulation: %w", err)
	}
	return nil
}

// WriteSimulationList renders the saved simulations.
func WriteSimulationList(w io.Writer, f Format, color bool, sims []app.SavedSimulation) error {
	if f == JSON {
		out := make([]savedSimulationDTO, len(sims))
		for i, s := range sims {
			out[i] = savedSimulationOf(s)
		}
		return WriteJSON(w, out)
	}
	rows := Rows{Header: []string{"ID", "NAME", "KIND"}}
	for _, s := range sims {
		rows.Body = append(rows.Body, []string{strconv.FormatInt(int64(s.ID), 10), s.Name, s.Kind})
	}
	if f == CSV {
		return WriteCSV(w, rows)
	}
	return WriteTable(w, rows, color)
}

// WriteSimulationCompare renders 2-3 reruns side by side, using the base rate of investment and
// PAC simulations.
func WriteSimulationCompare(
	w io.Writer, f Format, loc domain.Locale, color bool, cur domain.Currency, out simulating.CompareResult,
) error {
	if f == JSON {
		return WriteJSON(w, out)
	}
	m := &simMoney{cur: cur, loc: loc, csv: f == CSV}
	header := []string{""}
	cols := make([]map[string]string, len(out.Items))
	for i, it := range out.Items {
		header = append(header, it.Simulation.Name)
		cols[i] = compareValues(m, it)
	}
	if m.err != nil {
		return m.err
	}
	rows := Rows{Header: header}
	for c := 1; c < len(header); c++ {
		rows.MoneyCols = append(rows.MoneyCols, c)
	}
	for _, name := range []string{
		"Kind", "Contributed", "Nominal value", "Real value", "Net gain",
		"Opening balance", "First month below zero", "Difference at horizon", "Runway (months)",
	} {
		row := []string{name}
		present := false
		for _, col := range cols {
			v, ok := col[name]
			present = present || ok
			row = append(row, orDash(v))
		}
		if present {
			rows.Body = append(rows.Body, row)
		}
	}
	if f == CSV {
		return WriteCSV(w, rows)
	}
	return WriteTable(w, rows, color)
}

func compareValues(m *simMoney, it simulating.RerunResult) map[string]string {
	vals := map[string]string{"Kind": it.RunResult.Kind}
	t := it.RunResult.Investment
	if t == nil {
		t = it.RunResult.PAC
	}
	if t != nil {
		b := t.Base
		vals["Contributed"] = m.fmt(b.Contributions)
		vals["Nominal value"] = m.fmt(b.Nominal)
		vals["Real value"] = m.fmt(b.Real)
		vals["Net gain"] = m.fmt(b.NetGain)
	}
	if c := it.RunResult.CashFlow; c != nil {
		vals["Opening balance"] = m.fmt(c.OpeningMinor)
		vals["First month below zero"] = c.FirstBelowZero
		vals["Difference at horizon"] = m.fmt(c.EndDeltaMinor)
		vals["Runway (months)"] = runwayText(c.RunwayMonths)
	}
	return vals
}
