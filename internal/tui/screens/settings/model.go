package settings

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/theme"
)

// section identifies which part of the settings shell is active.
type section int

const (
	sectionModules section = iota
	sectionAppearance
)

// sectionNames are the section tabs in display order, matching the section constants above.
var sectionNames = [...]string{"Modules", "Appearance"}

// mode tracks whether the screen is browsing a section's list or awaiting a confirmation.
type mode int

const (
	modeList mode = iota
	modeConfirm
)

// Model is the settings screen: a shell with a Modules section (enable/disable modules, with a
// dependency warning when enabling and a confirmation when disabling an active dependent) and an
// Appearance section (a theme picker with a live preview). Update and View never touch the
// config file directly; every persistence call runs inside a tea.Cmd through moduleCfg/themeCfg.
type Model struct {
	uc        UseCases
	moduleCfg ModuleConfig
	themeCfg  ThemeConfig
	th        theme.Theme
	km        KeyMap

	width, height int
	section       section
	mode          mode

	enabled   module.Set
	modules   []app.ModuleInfo
	moduleTbl components.Table

	currentTheme string
	themeTbl     components.Table

	confirm        components.ConfirmDialog
	pendingDisable app.DisableModulePlan
}

var _ screens.Screen = (*Model)(nil)

// ModulesChangedMsg reports that the enabled module set changed and was persisted: the root
// composition layer is expected to rebuild the screen router and dashboard without a restart
// (see "Moduli" in the technical spec); this screen only emits the message.
type ModulesChangedMsg struct {
	Enabled module.Set
}

// ThemeChangedMsg reports that a theme was selected and persisted: the root re-themes the running
// TUI immediately instead of waiting for a restart.
type ThemeChangedMsg struct {
	Theme string
}

// ThemePreviewMsg asks the root to show a theme without persisting it: the cursor moved over it, or
// Esc restored the saved one.
type ThemePreviewMsg struct {
	Theme string
}

// New builds the settings screen over uc, moduleCfg (reads and persists modules.enabled), and
// themeCfg (reads and persists theme), rendering its own chrome with th.
func New(uc UseCases, moduleCfg ModuleConfig, themeCfg ThemeConfig, th theme.Theme) *Model {
	m := &Model{uc: uc, moduleCfg: moduleCfg, themeCfg: themeCfg, th: th, km: DefaultKeyMap()}
	m.moduleTbl = components.NewTable(moduleColumns(), th)
	m.themeTbl = components.NewTable(themeColumns(), th)
	m.confirm = components.NewConfirmDialog(th)
	return m
}

func moduleColumns() []components.Column {
	return []components.Column{
		{Title: "Module", Width: 14, Align: lipgloss.Left},
		{Title: "Enabled", Width: 8, Align: lipgloss.Left},
		{Title: "Depends on", Width: 28, Align: lipgloss.Left},
	}
}

func themeColumns() []components.Column {
	return []components.Column{
		{Title: "Theme", Width: 16, Align: lipgloss.Left},
		{Title: "", Width: 10, Align: lipgloss.Left},
	}
}

// SetTheme re-themes the screen in place, keeping its section, cursors and rows, so a live theme
// preview does not reset what the user is looking at.
func (m *Model) SetTheme(th theme.Theme) {
	m.th = th
	mc, tc := m.moduleTbl.Cursor(), m.themeTbl.Cursor()
	m.moduleTbl = components.NewTable(moduleColumns(), th)
	m.themeTbl = components.NewTable(themeColumns(), th)
	m.moduleTbl.SetRows(m.moduleRows())
	m.themeTbl.SetRows(m.themeRows())
	m.moduleTbl.SetCursor(mc)
	m.themeTbl.SetCursor(tc)
	if m.height > 0 {
		m.SetSize(m.width, m.height)
	}
	if m.mode != modeConfirm {
		m.confirm = components.NewConfirmDialog(th)
	}
}

// Init loads the enabled modules and the active theme.
func (m *Model) Init() tea.Cmd { return m.load() }

// SetSize adapts the section tables to the available terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	rows := max(3, h-10)
	m.moduleTbl.SetHeight(rows)
	m.moduleTbl.SetWidth(w)
	m.themeTbl.SetHeight(min(rows, len(themeNames)))
	m.themeTbl.SetWidth(w)
}

// Title identifies the screen in the tab bar.
func (m *Model) Title() string { return "Settings" }

// KeyMap exposes the bindings for the global help view.
func (m *Model) KeyMap() []key.Binding { return m.km.Bindings() }

// InputActive always reports false: the settings screen has no free-text field, only lists and a
// yes/no confirmation, none of which need to steal single-letter global bindings.
func (m *Model) InputActive() bool { return false }

// loadedMsg carries the enabled module set, module metadata, and the active theme back into
// Update.
type loadedMsg struct {
	enabled module.Set
	infos   []app.ModuleInfo
	theme   string
}

func (m *Model) load() tea.Cmd {
	return func() tea.Msg {
		enabled, err := m.moduleCfg.ModulesEnabled()
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		infos := m.uc.ListModules(context.Background(), enabled)
		th, err := m.themeCfg.Get("theme")
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return loadedMsg{enabled: enabled, infos: infos, theme: th}
	}
}

// modulesAppliedMsg carries the module set and metadata back into Update after an enable or
// disable call persists successfully.
type modulesAppliedMsg struct {
	enabled module.Set
	infos   []app.ModuleInfo
	notice  string
}

// disablePlannedMsg carries a disable plan back into Update, before it is applied, so dependent
// modules can be confirmed first.
type disablePlannedMsg struct {
	plan app.DisableModulePlan
}

// themeAppliedMsg carries the newly saved theme name back into Update.
type themeAppliedMsg struct {
	theme string
}

// Update advances the screen state; it performs no I/O itself.
func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.applyLoaded(msg)
		return m, nil
	case modulesAppliedMsg:
		m.enabled, m.modules = msg.enabled, msg.infos
		m.moduleTbl.SetRows(m.moduleRows())
		m.mode = modeList
		return m, tea.Batch(m.notify(msg.notice), emitModulesChanged(msg.enabled))
	case disablePlannedMsg:
		return m.handleDisablePlanned(msg.plan)
	case themeAppliedMsg:
		m.currentTheme = msg.theme
		m.themeTbl.SetRows(m.themeRows())
		m.mode = modeList
		name := msg.theme
		return m, tea.Batch(m.notify(fmt.Sprintf("theme saved: %s", name)),
			func() tea.Msg { return ThemeChangedMsg{Theme: name} })
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) applyLoaded(msg loadedMsg) {
	m.enabled, m.modules = msg.enabled, msg.infos
	m.moduleTbl.SetRows(m.moduleRows())
	m.currentTheme = msg.theme
	m.themeTbl.SetRows(m.themeRows())
	if i := slices.Index(themeNames, msg.theme); i >= 0 {
		m.themeTbl.SetCursor(i)
	}
	m.mode = modeList
}

func (m *Model) notify(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	return func() tea.Msg { return screens.StatusMsg{Text: text} }
}

func emitModulesChanged(enabled module.Set) tea.Cmd {
	return func() tea.Msg { return ModulesChangedMsg{Enabled: enabled} }
}

func (m *Model) handleKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	if m.mode == modeConfirm {
		return m.handleConfirmKey(msg)
	}
	switch {
	case key.Matches(msg, m.km.Section):
		restore := m.restoreTheme()
		m.section = (m.section + 1) % section(len(sectionNames))
		return m, restore
	case key.Matches(msg, m.km.Up):
		m.activeTable().MoveUp(1)
		return m, m.previewTheme()
	case key.Matches(msg, m.km.Down):
		m.activeTable().MoveDown(1)
		return m, m.previewTheme()
	case key.Matches(msg, m.km.Cancel):
		return m, m.restoreTheme()
	case key.Matches(msg, m.km.Toggle):
		return m.handleToggle()
	}
	return m, nil
}

// previewTheme shows the theme under the cursor when the Appearance section is active.
func (m *Model) previewTheme() tea.Cmd {
	if m.section != sectionAppearance {
		return nil
	}
	name := m.hoveredTheme()
	return func() tea.Msg { return ThemePreviewMsg{Theme: name} }
}

// restoreTheme drops an unsaved preview: the cursor returns to the saved theme and the root shows
// it again. It is a no-op when nothing is being previewed.
func (m *Model) restoreTheme() tea.Cmd {
	if m.section != sectionAppearance || m.hoveredTheme() == m.currentTheme {
		return nil
	}
	if i := slices.Index(themeNames, m.currentTheme); i >= 0 {
		m.themeTbl.SetCursor(i)
	}
	saved := m.currentTheme
	return func() tea.Msg { return ThemePreviewMsg{Theme: saved} }
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	yes, no := m.confirm.HandleKey(msg)
	switch {
	case yes:
		return m, m.applyDisable(m.pendingDisable)
	case no:
		m.mode = modeList
	}
	return m, nil
}

// activeTable returns the table owned by the current section.
func (m *Model) activeTable() *components.Table {
	if m.section == sectionAppearance {
		return &m.themeTbl
	}
	return &m.moduleTbl
}

func (m *Model) handleToggle() (screens.Screen, tea.Cmd) {
	switch m.section {
	case sectionAppearance:
		row := m.themeTbl.SelectedRow()
		if row == nil {
			return m, nil
		}
		return m, m.applyTheme(row.ID)
	case sectionModules:
		fallthrough
	default:
		row := m.moduleTbl.SelectedRow()
		if row == nil {
			return m, nil
		}
		id := module.ID(row.ID)
		if m.enabled.Has(id) {
			return m, m.planDisable(id)
		}
		return m, m.enableModule(id)
	}
}

func (m *Model) enableModule(id module.ID) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := m.uc.PlanEnableModule(ctx, m.enabled, id)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if len(plan.Added) == 0 {
			return modulesAppliedMsg{
				enabled: plan.After, infos: m.uc.ListModules(ctx, plan.After),
				notice: fmt.Sprintf("module %q is already enabled", id),
			}
		}
		if err := m.uc.ApplyEnableModule(ctx, m.moduleCfg, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return modulesAppliedMsg{enabled: plan.After, infos: m.uc.ListModules(ctx, plan.After), notice: enableNotice(plan)}
	}
}

func (m *Model) planDisable(id module.ID) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanDisableModule(context.Background(), m.enabled, id)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return disablePlannedMsg{plan: plan}
	}
}

func (m *Model) handleDisablePlanned(plan app.DisableModulePlan) (screens.Screen, tea.Cmd) {
	if !plan.RequiresConfirmation {
		return m, m.applyDisable(plan)
	}
	m.pendingDisable = plan
	deps := make([]string, len(plan.DependentsDisabled))
	for i, id := range plan.DependentsDisabled {
		deps[i] = string(id)
	}
	m.confirm.Show("Disable module", fmt.Sprintf("Disable %q?", plan.Target),
		"Also disables active dependents: "+strings.Join(deps, ", "))
	m.mode = modeConfirm
	return m, nil
}

func (m *Model) applyDisable(plan app.DisableModulePlan) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.uc.ApplyDisableModule(ctx, m.moduleCfg, plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return modulesAppliedMsg{enabled: plan.After, infos: m.uc.ListModules(ctx, plan.After), notice: disableNotice(plan)}
	}
}

func (m *Model) applyTheme(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.themeCfg.Set("theme", name); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return themeAppliedMsg{theme: name}
	}
}

// enableNotice summarizes an enable plan, warning when dependencies beyond the target were also
// enabled.
func enableNotice(plan app.EnableModulePlan) string {
	var extra []string
	for _, id := range plan.Added {
		if id != plan.Target {
			extra = append(extra, string(id))
		}
	}
	if len(extra) == 0 {
		return fmt.Sprintf("enabled module %q", plan.Target)
	}
	return fmt.Sprintf("enabled module %q (also enabled dependencies: %s)", plan.Target, strings.Join(extra, ", "))
}

// disableNotice summarizes a disable plan, warning when active dependents were also disabled.
func disableNotice(plan app.DisableModulePlan) string {
	if len(plan.DependentsDisabled) == 0 {
		return fmt.Sprintf("disabled module %q", plan.Target)
	}
	deps := make([]string, len(plan.DependentsDisabled))
	for i, id := range plan.DependentsDisabled {
		deps[i] = string(id)
	}
	return fmt.Sprintf("disabled module %q (also disabled dependents: %s)", plan.Target, strings.Join(deps, ", "))
}

func (m *Model) moduleRows() []components.Row {
	rows := make([]components.Row, len(m.modules))
	for i, info := range m.modules {
		enabled := "no"
		if info.Enabled {
			enabled = "yes"
		}
		deps := make([]string, len(info.Deps))
		for j, d := range info.Deps {
			deps[j] = string(d)
		}
		rows[i] = components.Row{ID: string(info.ID), Cells: []string{string(info.ID), enabled, strings.Join(deps, ", ")}}
	}
	return rows
}

func (m *Model) themeRows() []components.Row {
	rows := make([]components.Row, len(themeNames))
	for i, name := range themeNames {
		current := ""
		if name == m.currentTheme {
			current = "current"
		}
		rows[i] = components.Row{ID: name, Cells: []string{name, current}}
	}
	return rows
}

// View renders the active section and, when open, the disable confirmation dialog.
func (m *Model) View() string {
	out := m.viewSectionTabs() + "\n\n"
	if m.section == sectionAppearance {
		out += m.viewAppearance()
	} else {
		out += m.viewModules()
	}
	if m.mode == modeConfirm {
		out += "\n\n" + m.confirm.View()
	}
	return out
}

func (m *Model) viewSectionTabs() string {
	title := m.th.Title.Render("Settings")
	parts := make([]string, len(sectionNames))
	for i, name := range sectionNames {
		if section(i) == m.section {
			parts[i] = m.th.ActiveTab.Render(name)
		} else {
			parts[i] = m.th.InactiveTab.Render(name)
		}
	}
	return title + "\n" + strings.Join(parts, "  ")
}

func (m *Model) viewModules() string {
	return m.th.Header.Render("Enabled modules") + "\n" + m.moduleTbl.View()
}

func (m *Model) viewAppearance() string {
	name := m.hoveredTheme()
	preview := theme.Resolve(theme.Config{Theme: name}, nil)
	sample := preview.Title.Render("Sample title") + "\n" +
		preview.Positive.Render("+ income") + "  " + preview.Negative.Render("- expense") + "\n" +
		preview.Warning.Render("warning") + "  " + preview.Muted.Render("muted text")
	box := preview.Border.Render(sample)
	return m.th.Header.Render("Theme") + "\n" + m.themeTbl.View() + "\n\n" +
		m.th.Header.Render("Preview: "+name) + "\n" + box
}

// hoveredTheme returns the theme name under the cursor, falling back to the currently saved
// theme when the table has no rows yet.
func (m *Model) hoveredTheme() string {
	if row := m.themeTbl.SelectedRow(); row != nil {
		return row.ID
	}
	return m.currentTheme
}
