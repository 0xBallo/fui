package report

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/domain/simulate"
	"github.com/0xballo/fui/internal/tui/components"
)

func simTripleColumns() []components.Column {
	return []components.Column{
		{Title: "Scenario", Width: 14, Align: lipgloss.Left},
		{Title: "Contributions", Width: 16, Align: lipgloss.Right},
		{Title: "Nominal", Width: 16, Align: lipgloss.Right},
		{Title: "Real", Width: 16, Align: lipgloss.Right},
		{Title: "Net gain", Width: 16, Align: lipgloss.Right},
	}
}

func simCashFlowColumns() []components.Column {
	return []components.Column{
		{Title: "Month end", Width: 12, Align: lipgloss.Left},
		{Title: "Baseline", Width: 16, Align: lipgloss.Right},
		{Title: "Scenario", Width: 16, Align: lipgloss.Right},
		{Title: "Delta", Width: 16, Align: lipgloss.Right},
	}
}

// simAccountColumns shows the scenario balance of the total and of each selected account.
func simAccountColumns(accounts []simulating.CashFlowAccount) []components.Column {
	cols := []components.Column{
		{Title: "Month end", Width: 12, Align: lipgloss.Left},
		{Title: "Total", Width: 14, Align: lipgloss.Right},
	}
	for _, a := range accounts {
		cols = append(cols, components.Column{Title: a.Account, Width: 14, Align: lipgloss.Right})
	}
	return cols
}

func (m *Model) simAccountRows(r *simulating.CashFlowResult) []components.Row {
	rows := make([]components.Row, len(r.Months))
	for i, mo := range r.Months {
		cells := []string{mo.End, m.simMoney(mo.ScenarioMinor)}
		for _, a := range r.Accounts {
			cell := ""
			if i < len(a.Months) {
				cell = m.simMoney(a.Months[i].ScenarioMinor)
			}
			cells = append(cells, cell)
		}
		rows[i] = components.Row{ID: mo.End, Cells: cells}
	}
	return rows
}

func simCompareColumns() []components.Column {
	return []components.Column{
		{Title: "Simulation", Width: 20, Align: lipgloss.Left},
		{Title: "Kind", Width: 12, Align: lipgloss.Left},
		{Title: "Pessimistic", Width: 16, Align: lipgloss.Right},
		{Title: "Base", Width: 16, Align: lipgloss.Right},
		{Title: "Optimistic", Width: 16, Align: lipgloss.Right},
	}
}

func (m *Model) simMoney(minor int64) string {
	money, err := domain.NewMoney(minor, m.sim.uc.BaseCurrency())
	if err != nil {
		return "?"
	}
	return domain.FormatAmount(money, m.loc)
}

func (m *Model) simSigned(minor int64) string {
	s := m.simMoney(minor)
	if minor > 0 {
		return "+" + s
	}
	return s
}

func (m *Model) applySimRun(msg simRunMsg) {
	s := &m.sim
	s.lastReq, s.result, s.saved = msg.req, &msg.result, msg.saved
	s.panel = simPanelRun
	s.note = ""
	switch {
	case msg.result.CashFlow != nil:
		m.applySimCashFlow(msg.result.CashFlow)
	case msg.result.Investment != nil:
		m.applySimTriple(msg.result.Investment)
	case msg.result.PAC != nil:
		m.applySimTriple(msg.result.PAC)
	}
	if msg.req != nil {
		if r := simRates(*msg.req); r != nil {
			s.note = "Annual rates: " + bpText(r.PessimisticBP) + " / " + bpText(r.BaseBP) + " / " + bpText(r.OptimisticBP)
		}
	}
}

func simRates(req simulating.RunRequest) *simulating.Rates {
	switch {
	case req.Investment != nil:
		return &req.Investment.Rates
	case req.PAC != nil:
		return &req.PAC.Rates
	}
	return nil
}

func bpText(bp int64) string {
	sign := ""
	if bp < 0 {
		sign, bp = "-", -bp
	}
	return sign + strconv.FormatInt(bp/100, 10) + "." + pad2(bp%100) + "%"
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func (m *Model) applySimTriple(t *simulate.Triple) {
	row := func(label string, r simulate.Result) components.Row {
		return components.Row{ID: label, Cells: []string{
			label, m.simMoney(r.Contributions), m.simMoney(r.Nominal), m.simMoney(r.Real), m.simSigned(r.NetGain),
		}}
	}
	m.sim.table.SetColumns(simTripleColumns())
	m.sim.table.SetRows([]components.Row{
		row("Pessimistic", t.Pessimistic), row("Base", t.Base), row("Optimistic", t.Optimistic),
	})
}

func (m *Model) applySimCashFlow(r *simulating.CashFlowResult) {
	rows := make([]components.Row, len(r.Months))
	for i, mo := range r.Months {
		rows[i] = components.Row{ID: mo.End, Cells: []string{
			mo.End, m.simMoney(mo.BaseMinor), m.simMoney(mo.ScenarioMinor), m.simSigned(mo.ScenarioMinor - mo.BaseMinor),
		}}
	}
	m.sim.table.SetColumns(simCashFlowColumns())
	if len(r.Accounts) > 0 {
		m.sim.table.SetColumns(simAccountColumns(r.Accounts))
		rows = m.simAccountRows(r)
	}
	m.sim.table.SetRows(rows)
	note := "Opening balance " + m.simMoney(r.OpeningMinor) + "; end delta " + m.simSigned(r.EndDeltaMinor)
	if r.FirstBelowZero != "" {
		note += "; first below zero " + r.FirstBelowZero
	}
	m.sim.note = note
}

func (m *Model) refreshSimCompare() {
	rows := make([]components.Row, len(m.sim.compare))
	for i, it := range m.sim.compare {
		name, kind := it.Simulation.Name, it.Simulation.Kind
		cells := []string{name, kind, "—", "—", "—"}
		switch r := it.RunResult; {
		case r.Investment != nil:
			cells[2], cells[3], cells[4] = m.tripleNominal(r.Investment)
		case r.PAC != nil:
			cells[2], cells[3], cells[4] = m.tripleNominal(r.PAC)
		case r.CashFlow != nil && len(r.CashFlow.Months) > 0:
			cells[3] = m.simMoney(r.CashFlow.Months[len(r.CashFlow.Months)-1].ScenarioMinor)
		}
		rows[i] = components.Row{ID: name, Cells: cells}
	}
	m.sim.cmpTable.SetRows(rows)
}

func (m *Model) tripleNominal(t *simulate.Triple) (low, base, high string) {
	return m.simMoney(t.Pessimistic.Nominal), m.simMoney(t.Base.Nominal), m.simMoney(t.Optimistic.Nominal)
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline draws vals as one block character each, scaled between their minimum and maximum.
func sparkline(vals []int64) string {
	if len(vals) == 0 {
		return ""
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		lo, hi = min(lo, v), max(hi, v)
	}
	var b strings.Builder
	for _, v := range vals {
		idx := 0
		if hi > lo {
			idx = int((v - lo) * int64(len(sparkBlocks)-1) / (hi - lo))
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}

// simSeries is the series the sparkline shows: yearly nominal value of the base scenario, or the
// monthly balance of a cash-flow scenario.
func (m *Model) simSeries() (label string, vals []int64) {
	r := m.sim.result
	if r == nil {
		return "", nil
	}
	switch {
	case r.CashFlow != nil:
		for _, mo := range r.CashFlow.Months {
			vals = append(vals, mo.ScenarioMinor)
		}
		return "Balance by month", vals
	case r.Investment != nil:
		return "Base value by year", yearlyNominal(r.Investment.Base)
	case r.PAC != nil:
		return "Base value by year", yearlyNominal(r.PAC.Base)
	}
	return "", nil
}

func yearlyNominal(r simulate.Result) []int64 {
	vals := make([]int64, len(r.Rows))
	for i, row := range r.Rows {
		vals[i] = row.Nominal
	}
	return vals
}

func (m *Model) viewSim() string {
	switch m.sim.panel {
	case simPanelForm:
		return components.Modal(m.th, m.width, m.height,
			m.th.Title.Render("Simulation params — "+simKinds[m.sim.kindIdx])+"\n\n"+m.sim.form.View())
	case simPanelSave:
		return components.Modal(m.th, m.width, m.height, m.th.Title.Render("Save simulation")+"\n\n"+m.sim.form.View())
	case simPanelConfirm:
		return components.Modal(m.th, m.width, m.height, m.confirm.View())
	case simPanelSaved:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.th.Title.Render("Saved simulations"), "", m.sim.savedTable.View(), "",
			m.th.Base.Render("space select (max 3)   c compare   enter show   d delete   esc back"))
	case simPanelCompare:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.th.Title.Render("Compare simulations — final nominal value"), "", m.sim.cmpTable.View(), "",
			m.th.Base.Render("esc back"))
	case simPanelRun:
		return m.viewSimRun()
	}
	return m.viewSimRun()
}

func (m *Model) viewSimRun() string {
	title := "Reports — Simulation: " + simKinds[m.sim.kindIdx]
	if m.sim.saved != "" {
		title = "Reports — Simulation (saved: " + m.sim.saved + ")"
	}
	lines := []string{m.th.Title.Render(title), "", m.sim.table.View()}
	if label, vals := m.simSeries(); len(vals) > 0 {
		lines = append(lines, "", m.th.Muted.Render(label+": ")+sparkline(vals))
	}
	if m.sim.note != "" {
		lines = append(lines, "", m.th.Muted.Render(m.sim.note))
	}
	lines = append(lines, "", m.th.Base.Render("h/l type   p params   r run   s save   v saved   esc back"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
