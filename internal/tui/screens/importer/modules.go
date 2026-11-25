package importer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
)

// pendingModules lists the modules the user chose to enable, in plan order.
func (m *Model) pendingModules() []module.ID {
	var ids []module.ID
	for _, r := range m.plan.ModuleRecords {
		if m.enableModules[r.Module] {
			ids = append(ids, r.Module)
		}
	}
	return ids
}

// applyOptions carries the module and transfer choices into ApplyImportWith; the transfers the
// user accepted travel inside the plan itself (ImportPlan.SetAcceptTransfer).
func (m *Model) applyOptions() importing.ApplyOptions {
	opts := importing.ApplyOptions{Modules: m.modules}
	if ids := m.pendingModules(); len(ids) > 0 {
		opts.EnableModules = ids
		opts.ModuleWriter = m.moduleWriter
	}
	return opts
}

// afterModulesEnabled mirrors the modules just enabled by the import into the local set.
func (m *Model) afterModulesEnabled() {
	for _, id := range m.pendingModules() {
		if next, _, err := module.Enable(m.modules, id); err == nil {
			m.modules = next
		}
	}
	m.enableModules = map[module.ID]bool{}
}

// openModules opens the dialog listing the disabled modules whose records the stream holds.
func (m *Model) openModules() (screens.Screen, tea.Cmd) {
	if len(m.plan.ModuleRecords) == 0 {
		m.notice = "No records of disabled modules."
		return m, nil
	}
	m.notice = ""
	m.moduleCursor = 0
	m.mode = modeModules
	return m, nil
}

func (m *Model) handleModulesKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	n := len(m.plan.ModuleRecords)
	switch {
	case key.Matches(msg, m.km.Up):
		m.moduleCursor = max(0, m.moduleCursor-1)
	case key.Matches(msg, m.km.Down):
		m.moduleCursor = min(n-1, m.moduleCursor+1)
	case key.Matches(msg, m.km.Toggle):
		if m.moduleWriter == nil {
			m.notice = "Enabling modules is not available here: use `fui module enable`."
			return m, nil
		}
		id := m.plan.ModuleRecords[m.moduleCursor].Module
		m.enableModules[id] = !m.enableModules[id]
	case key.Matches(msg, m.km.Confirm):
		m.mode = modePreview
	case key.Matches(msg, m.km.Back):
		m.enableModules = map[module.ID]bool{}
		m.mode = modePreview
	}
	return m, nil
}

func (m *Model) viewModules() string {
	out := m.th.Title.Render("Disabled modules") + "\n"
	out += m.th.Base.Render("Enable a module to import its records; the others are skipped.") + "\n\n"
	rows := make([]string, len(m.plan.ModuleRecords))
	for i, r := range m.plan.ModuleRecords {
		cursor, mark := "  ", "[ ] skip  "
		if i == m.moduleCursor {
			cursor = "> "
		}
		if m.enableModules[r.Module] {
			mark = "[x] enable"
		}
		rows[i] = fmt.Sprintf("%s%s %s: %d record(s) [%s]", cursor, mark, r.Module, r.Count, strings.Join(r.Kinds, ","))
	}
	out += m.th.Base.Render(strings.Join(rows, "\n"))
	out += "\n\n" + m.th.Muted.Render("j/k move · space enable/skip · enter done · esc cancel")
	if m.notice != "" {
		out += "\n" + m.th.Warning.Render(m.notice)
	}
	return out
}
