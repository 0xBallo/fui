package transactions

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// searchDebounce is how long the quick search box waits after the last keystroke before it
// queries, so fast typing only runs one query against the final text.
const searchDebounce = 150 * time.Millisecond

// debounceMsg fires searchDebounce after a keystroke; seq pins it to the field value at the time
// of that keystroke, so Update can drop it once a later keystroke has moved seq on.
type debounceMsg struct{ seq int }

// searchResultMsg carries the outcome of one debounced search query back into Update. seq lets
// Update drop results from a query a later keystroke has already superseded.
type searchResultMsg struct {
	seq  int
	text string
	page app.TxPage
	err  error
}

// defaultTick is the production debounce timer: a real tea.Tick after searchDebounce. Tests
// substitute Model.tick with a function that fires immediately, so debounce/cancel sequencing can
// be asserted deterministically without sleeping.
func defaultTick(seq int) tea.Cmd {
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg { return debounceMsg{seq: seq} })
}

func (m *Model) openSearch() {
	m.searchField = components.NewAutocomplete("Search:", "description contains…", m.th)
	m.searchField.SetValue(m.req.Text)
	m.mode = modeSearch
	m.searchField.Focus()
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelSearch()
		m.searchField.SetValue("")
		m.req.Text = ""
		m.mode = modeList
		return m, m.load()
	case "enter":
		m.cancelSearch()
		m.req.Text = m.searchField.Value()
		m.mode = modeList
		return m, m.load()
	}
	updated, _ := m.searchField.Update(msg)
	m.searchField = updated
	return m, m.restartSearch()
}

func (m *Model) viewSearch() string {
	return m.viewList() + "\n\n" + m.searchField.View()
}

// restartSearch bumps the search sequence, cancels any in-flight query (one started by a previous
// keystroke that hasn't returned yet) and arms a fresh debounce timer for the new sequence.
func (m *Model) restartSearch() tea.Cmd {
	m.searchSeq++
	seq := m.searchSeq
	m.cancelSearch()
	return m.tick(seq)
}

// cancelSearch cancels the context of an in-flight search query, if any.
func (m *Model) cancelSearch() {
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}
}

// runSearch starts the ListTx call for seq, over the search field's current text, in a cancelable
// context stored on the model so a later keystroke can cancel it.
func (m *Model) runSearch(seq int) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.searchCancel = cancel
	uc := m.uc
	req := m.req
	text := m.searchField.Value()
	req.Text = text
	req.Limit = pageSize
	req.After = nil
	return func() tea.Msg {
		page, err := uc.ListTx(ctx, req)
		return searchResultMsg{seq: seq, text: text, page: page, err: err}
	}
}

// applySearchResult applies a debounced query's result, dropping it if a later keystroke has
// since moved the search sequence on (including a canceled context, which arrives as an error).
func (m *Model) applySearchResult(msg searchResultMsg) (screens.Screen, tea.Cmd) {
	if msg.seq != m.searchSeq {
		return m, nil
	}
	m.searchCancel = nil
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			return m, nil
		}
		return m, func() tea.Msg { return screens.ErrorMsg{Err: msg.err} }
	}
	m.req.Text = msg.text
	m.items = msg.page.Items
	m.next = msg.page.Next
	m.table.SetRows(m.txRows())
	m.lastErr = nil
	return m, nil
}
