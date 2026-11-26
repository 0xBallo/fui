package report

import (
	"context"
	"errors"
	"slices"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// maxCompare is how many saved simulations can be compared side by side.
const maxCompare = 3

// SimUseCases is what the simulations view needs; it matches internal/app/simulating.Service with
// the enabled-module set already bound, plus the base currency amounts are entered and shown in.
// It is separate from UseCases so the view stays optional: without it the "m" key is not offered.
type SimUseCases interface {
	Run(ctx context.Context, req simulating.RunRequest) (simulating.RunResult, error)
	PlanSave(ctx context.Context, name string, req simulating.RunRequest) (simulating.SavePlan, error)
	ApplySave(ctx context.Context, p simulating.SavePlan) (app.SavedSimulation, error)
	List(ctx context.Context) ([]app.SavedSimulation, error)
	PlanRemove(ctx context.Context, ref string) (simulating.RemovePlan, error)
	ApplyRemove(ctx context.Context, p simulating.RemovePlan) error
	Rerun(ctx context.Context, ref string) (simulating.RerunResult, error)
	Compare(ctx context.Context, refs []string) (simulating.CompareResult, error)
	BaseCurrency() domain.Currency
}

// simKeys are the bindings of the simulations view.
type simKeys struct {
	Open, Params, Save, Saved, Run, Compare, Toggle, Remove, Back key.Binding
	PrevKind, NextKind                                            key.Binding
}

func defaultSimKeys() simKeys {
	return simKeys{
		Open:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "simulations")),
		PrevKind: key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "previous type")),
		NextKind: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "next type")),
		Params:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "edit params")),
		Run:      key.NewBinding(key.WithKeys("r", "enter"), key.WithHelp("r/enter", "run")),
		Save:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save simulation")),
		Saved:    key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "saved simulations")),
		Compare:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compare selected")),
		Toggle:   key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
		Remove:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete saved simulation")),
		Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// simPanel is the active panel inside the simulations mode.
type simPanel int

const (
	simPanelRun simPanel = iota
	simPanelForm
	simPanelSave
	simPanelSaved
	simPanelCompare
	simPanelConfirm
)

// simKinds are the simulation types in the order they are cycled.
var simKinds = []string{simulating.KindInvestment, simulating.KindPAC, simulating.KindCashFlow}

// simState is the optional simulations view of the report screen.
type simState struct {
	uc    SimUseCases
	km    simKeys
	panel simPanel

	kindIdx int
	values  map[string]string

	lastReq *simulating.RunRequest // what the result shown was run with; nil after a rerun of a saved one
	result  *simulating.RunResult
	saved   string // saved simulation name of the result shown, "" for an ad-hoc run

	table components.Table

	fields []components.Autocomplete
	form   components.Form

	savedList  []app.SavedSimulation
	savedTable components.Table
	compare    []simulating.RerunResult
	cmpTable   components.Table
	note       string
}

// simRunMsg carries a simulation result back into Update.
type simRunMsg struct {
	req    *simulating.RunRequest
	result simulating.RunResult
	saved  string
}

// simListMsg carries the saved simulations back into Update.
type simListMsg struct{ rows []app.SavedSimulation }

// simCompareMsg carries a comparison of saved simulations back into Update.
type simCompareMsg struct{ result simulating.CompareResult }

// SetSimulations enables the simulations view over uc; nil disables it.
func (m *Model) SetSimulations(uc SimUseCases) {
	m.sim = simState{uc: uc, km: defaultSimKeys(), values: defaultSimValues()}
	m.sim.table = components.NewTable(nil, m.th)
	m.sim.savedTable = components.NewTable(simSavedColumns(), m.th)
	m.sim.savedTable.SetShowSelect(true)
	m.sim.cmpTable = components.NewTable(simCompareColumns(), m.th)
	m.sizeSim()
}

func (m *Model) sizeSim() {
	h := max(3, m.height-12)
	for _, t := range []*components.Table{&m.sim.table, &m.sim.savedTable, &m.sim.cmpTable} {
		t.SetHeight(h)
		t.SetWidth(m.width)
	}
}

func (m *Model) simEnabled() bool { return m.sim.uc != nil }

func (m *Model) simBindings() []key.Binding {
	if !m.simEnabled() {
		return nil
	}
	if m.mode == modeSim && m.sim.panel == simPanelSaved {
		return []key.Binding{m.sim.km.Open, m.sim.km.Remove}
	}
	return []key.Binding{m.sim.km.Open}
}

func (m *Model) simInputActive() bool {
	return m.mode == modeSim && (m.sim.panel == simPanelForm || m.sim.panel == simPanelSave)
}

func (m *Model) openSim() tea.Cmd {
	m.mode = modeSim
	m.sim.panel = simPanelRun
	return m.runSimForm()
}

// updateSim consumes the messages of the simulations view; ok is false for any other message.
func (m *Model) updateSim(msg tea.Msg) (cmd tea.Cmd, ok bool) {
	switch msg := msg.(type) {
	case simRunMsg:
		m.applySimRun(msg)
	case simListMsg:
		m.sim.savedList = msg.rows
		m.refreshSimSaved()
		m.sim.panel = simPanelSaved
	case simCompareMsg:
		m.sim.compare = msg.result.Items
		m.refreshSimCompare()
		m.sim.panel = simPanelCompare
	default:
		return nil, false
	}
	return nil, true
}

func (m *Model) simErr(err error) tea.Cmd {
	return func() tea.Msg { return screens.ErrorMsg{Err: err} }
}

// runSimForm runs the simulation described by the current form values.
func (m *Model) runSimForm() tea.Cmd {
	req, err := buildSimRequest(simKinds[m.sim.kindIdx], m.sim.values, m.loc, m.sim.uc.BaseCurrency())
	if err != nil {
		return m.simErr(err)
	}
	uc := m.sim.uc
	return func() tea.Msg {
		res, err := uc.Run(context.Background(), req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return simRunMsg{req: &req, result: res}
	}
}

func (m *Model) handleSimKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch m.sim.panel {
	case simPanelForm:
		return m.handleSimFormKey(msg)
	case simPanelSave:
		return m.handleSimSaveKey(msg)
	case simPanelSaved:
		return m.handleSimSavedKey(msg)
	case simPanelConfirm:
		return m.handleSimConfirmKey(msg)
	case simPanelCompare:
		if key.Matches(msg, m.sim.km.Back) {
			m.sim.panel = simPanelSaved
		}
		return m, nil
	case simPanelRun:
		return m.handleSimRunKey(msg)
	}
	return m, nil
}

func (m *Model) handleSimRunKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	k := m.sim.km
	switch {
	case key.Matches(msg, k.Back):
		m.mode = modeReport
	case key.Matches(msg, m.km.Up):
		m.sim.table.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		m.sim.table.MoveDown(1)
	case key.Matches(msg, k.PrevKind):
		return m, m.cycleSimKind(-1)
	case key.Matches(msg, k.NextKind):
		return m, m.cycleSimKind(1)
	case key.Matches(msg, k.Run):
		return m, m.runSimForm()
	case key.Matches(msg, k.Params):
		m.openSimForm()
	case key.Matches(msg, k.Save):
		if m.sim.lastReq == nil {
			return m, m.simErr(errors.New("run a simulation with p/r before saving it"))
		}
		m.openSimSave()
	case key.Matches(msg, k.Saved):
		return m, m.loadSimSaved()
	}
	return m, nil
}

func (m *Model) cycleSimKind(delta int) tea.Cmd {
	n := len(simKinds)
	m.sim.kindIdx = (m.sim.kindIdx + delta + n) % n
	return m.runSimForm()
}

func (m *Model) loadSimSaved() tea.Cmd {
	uc := m.sim.uc
	return func() tea.Msg {
		rows, err := uc.List(context.Background())
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return simListMsg{rows: rows}
	}
}

func (m *Model) handleSimSavedKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	k := m.sim.km
	t := &m.sim.savedTable
	switch {
	case key.Matches(msg, k.Back):
		m.sim.panel = simPanelRun
	case key.Matches(msg, m.km.Up):
		t.MoveUp(1)
	case key.Matches(msg, m.km.Down):
		t.MoveDown(1)
	case key.Matches(msg, k.Toggle):
		row := t.SelectedRow()
		if row == nil {
			return m, nil
		}
		if len(t.SelectedIDs()) >= maxCompare && !slices.Contains(t.SelectedIDs(), row.ID) {
			return m, m.simErr(errors.New("at most 3 simulations can be compared"))
		}
		t.ToggleSelect()
	case key.Matches(msg, k.Compare):
		refs := m.selectedSimRefs()
		if len(refs) < 2 {
			return m, m.simErr(errors.New("select 2 or 3 simulations with space to compare"))
		}
		return m, m.compareSim(refs)
	case key.Matches(msg, k.Run):
		if row := t.SelectedRow(); row != nil {
			return m, m.rerunSim(row.ID)
		}
	case key.Matches(msg, k.Remove):
		if row := t.SelectedRow(); row != nil {
			m.confirm.Show("Delete simulation", "Delete saved simulation \""+row.ID+"\"?")
			m.sim.panel = simPanelConfirm
		}
	}
	return m, nil
}

func (m *Model) handleSimConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		m.sim.panel = simPanelSaved
		return m, m.removeSim(m.sim.savedTable.SelectedRow().ID)
	case no:
		m.sim.panel = simPanelSaved
	}
	return m, nil
}

// removeSim deletes a saved simulation and reloads the list.
func (m *Model) removeSim(ref string) tea.Cmd {
	uc := m.sim.uc
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanRemove(ctx, ref)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := uc.ApplyRemove(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		rows, err := uc.List(ctx)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return simListMsg{rows: rows}
	}
}

func (m *Model) compareSim(refs []string) tea.Cmd {
	uc := m.sim.uc
	return func() tea.Msg {
		out, err := uc.Compare(context.Background(), refs)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return simCompareMsg{result: out}
	}
}

func (m *Model) rerunSim(ref string) tea.Cmd {
	uc := m.sim.uc
	return func() tea.Msg {
		out, err := uc.Rerun(context.Background(), ref)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return simRunMsg{result: out.RunResult, saved: out.Simulation.Name}
	}
}

func (m *Model) openSimSave() {
	m.sim.fields = []components.Autocomplete{components.NewAutocomplete("Name:", "e.g. retirement", m.th)}
	m.sim.form = components.NewForm(&m.sim.fields[0])
	m.sim.panel = simPanelSave
	m.sim.form.Focus()
}

func (m *Model) handleSimSaveKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.sim.panel = simPanelRun
	case "enter":
		if m.sim.form.FocusedOpen() {
			m.sim.form.Update(msg)
			return m, nil
		}
		name := m.sim.fields[0].Value()
		m.sim.panel = simPanelRun
		return m, m.saveSim(name, *m.sim.lastReq)
	default:
		m.sim.form.Update(msg)
	}
	return m, nil
}

func (m *Model) saveSim(name string, req simulating.RunRequest) tea.Cmd {
	uc := m.sim.uc
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanSave(ctx, name, req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if _, err := uc.ApplySave(ctx, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return screens.StatusMsg{Text: "Saved simulation \"" + name + "\"."}
	}
}

func simSavedColumns() []components.Column {
	return []components.Column{
		{Title: "Name", Width: 24, Align: lipgloss.Left},
		{Title: "Kind", Width: 12, Align: lipgloss.Left},
		{Title: "Params", Width: 40, Align: lipgloss.Left},
	}
}

func (m *Model) refreshSimSaved() {
	rows := make([]components.Row, len(m.sim.savedList))
	for i, s := range m.sim.savedList {
		rows[i] = components.Row{ID: s.Name, Cells: []string{s.Name, s.Kind, s.ParamsJSON}}
	}
	m.sim.savedTable.ClearSelection()
	m.sim.savedTable.SetRows(rows)
}

// selectedSimRefs returns the marked simulations in list order, so the comparison is stable.
func (m *Model) selectedSimRefs() []string {
	marked := m.sim.savedTable.SelectedIDs()
	var refs []string
	for _, s := range m.sim.savedList {
		if slices.Contains(marked, s.Name) {
			refs = append(refs, s.Name)
		}
	}
	return refs
}
