package lists

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// runState is the "run until" prompt: a single date field plus the preview of the plan computed
// for the date the user last asked about.
type runState struct {
	field   components.Autocomplete
	preview *recurring.RunPlan // nil until a plan matches the field
	until   string             // the field value preview was computed for
}

type recurrenceRunPlannedMsg struct {
	until string
	plan  recurring.RunPlan
}

type recurrenceRunAppliedMsg struct {
	count  int
	loaded recurrencesLoadedMsg
}

// openRun opens the prompt and previews the default (today, an empty Until), so the user sees the
// count before touching anything.
func (m *Recurrences) openRun() tea.Cmd {
	m.err = nil
	m.run = runState{field: components.NewAutocomplete("Run until:", "YYYY-MM-DD (empty = today)", m.th)}
	m.run.field.Focus()
	m.mode = recurrencesRun
	return m.planRun("")
}

func (m *Recurrences) planRun(until string) tea.Cmd {
	return func() tea.Msg {
		plan, err := m.uc.PlanRun(context.Background(), m.enabled, recurring.RunRequest{Until: until})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return recurrenceRunPlannedMsg{until: until, plan: plan}
	}
}

func (m *Recurrences) applyRun() tea.Cmd {
	plan := *m.run.preview
	return func() tea.Msg {
		ctx := context.Background()
		n, err := m.uc.ApplyRun(ctx, plan)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		loaded, ok := m.reload(ctx).(recurrencesLoadedMsg)
		if !ok {
			return screens.ErrorMsg{Err: fmt.Errorf("reload recurrences after run")}
		}
		return recurrenceRunAppliedMsg{count: n, loaded: loaded}
	}
}

// handleRunKey: enter plans the typed date first and applies only when the preview already matches
// it, so the count shown is always the count that gets created.
func (m *Recurrences) handleRunKey(msg tea.KeyPressMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.err = nil
		m.mode = recurrencesListMode
		return m, nil
	case "enter":
		until := strings.TrimSpace(m.run.field.Value())
		if m.run.preview != nil && m.run.until == until {
			return m, m.applyRun()
		}
		return m, m.planRun(until)
	}
	m.run.field.Update(msg)
	return m, nil
}

func (m *Recurrences) viewRun() string {
	body := m.th.Title.Render("Run recurrences") + "\n\n" + m.run.field.View() + "\n\n"
	if m.run.preview == nil {
		return body + m.th.Muted.Render("enter: preview")
	}
	return body + fmt.Sprintf("%d occurrence(s) will be created as pending.\n", len(m.run.preview.Created)) +
		m.th.Muted.Render("enter: confirm · esc: cancel")
}
