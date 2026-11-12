package transactions

import (
	"context"
	"errors"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

// instantTick replaces Model.tick in tests: it fires the debounce message immediately instead of
// sleeping, so sequencing can be driven deterministically by choosing when to run each returned
// tea.Cmd instead of waiting on the real clock.
func instantTick(seq int) tea.Cmd {
	return func() tea.Msg { return debounceMsg{seq: seq} }
}

// recordingUseCases wraps a UseCases, recording the Text of every ListTx call it forwards.
type recordingUseCases struct {
	UseCases
	mu    sync.Mutex
	texts []string
}

func (r *recordingUseCases) ListTx(ctx context.Context, req app.ListTxRequest) (app.TxPage, error) {
	r.mu.Lock()
	r.texts = append(r.texts, req.Text)
	r.mu.Unlock()
	return r.UseCases.ListTx(ctx, req)
}

func (r *recordingUseCases) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.texts...)
}

// TestSearchDebounceOnlyQueriesFinalText types a query one keystroke at a time without running any
// debounce timer in between (simulating keystrokes faster than the 150ms debounce), then runs
// every pending tick in the order the keystrokes produced them: every one but the last carries a
// sequence number Update has since moved past, so it must be dropped, leaving exactly one
// additional ListTx call reaching the port, with the final text.
func TestSearchDebounceOnlyQueriesFinalText(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-5", Description: "Coffee", Date: "2026-06-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Groceries run", Date: "2026-06-02"})

	rec := &recordingUseCases{UseCases: fx.Service}
	m := New(rec, domain.LocaleENUS, theme.Default())
	m.tick = instantTick
	m.SetSize(100, 30)
	m = drain(m, m.Init())

	m.openSearch()
	var ticks []tea.Cmd
	for _, r := range "Coffee" {
		screen, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = screen.(*Model)
		ticks = append(ticks, cmd)
	}

	before := len(rec.snapshot())
	for _, cmd := range ticks {
		m = drain(m, cmd)
	}

	got := rec.snapshot()[before:]
	if len(got) != 1 || got[0] != "Coffee" {
		t.Fatalf("expected exactly one query for %q, got %v", "Coffee", got)
	}
}

// TestSearchResultSetsReqTextForLoadMore reproduces F1: a live search (debounced, so it never
// goes through handleSearchKey's enter path, which used to be the only place that set
// m.req.Text) must still leave req.Text set once the result is applied, so a subsequent
// loadMore/reload keeps querying with the searched text instead of silently reverting to "".
func TestSearchResultSetsReqTextForLoadMore(t *testing.T) {
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "100", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-5", Description: "Coffee", Date: "2026-06-01"})

	rec := &recordingUseCases{UseCases: fx.Service}
	m := New(rec, domain.LocaleENUS, theme.Default())
	m.tick = instantTick
	m.SetSize(100, 30)
	m = drain(m, m.Init())

	m.openSearch()
	screen, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Coffee")})
	m = screen.(*Model)
	m = drain(m, cmd)

	if m.req.Text != "Coffee" {
		t.Fatalf("expected req.Text to be set from the live search result, got %q", m.req.Text)
	}

	before := len(rec.snapshot())
	drain(m, m.loadMore())
	got := rec.snapshot()[before:]
	if len(got) != 1 || got[0] != "Coffee" {
		t.Fatalf("expected loadMore to keep querying with %q, got %v", "Coffee", got)
	}
}

// drain runs cmd and every tea.Cmd Update returns in reaction, until none remain, the way the
// production Bubble Tea runtime would, returning the settled Model.
func drain(m *Model, cmd tea.Cmd) *Model {
	for cmd != nil {
		msg := cmd()
		screen, next := m.Update(msg)
		m = screen.(*Model)
		cmd = next
	}
	return m
}

// ctxUseCases is a minimal UseCases stub (every other method is unreachable in these tests) whose
// ListTx reports the context's own error, so tests can observe whether runSearch's context was
// canceled by a later keystroke.
type ctxUseCases struct{ UseCases }

func (ctxUseCases) ListTx(ctx context.Context, _ app.ListTxRequest) (app.TxPage, error) {
	return app.TxPage{}, ctx.Err()
}

// TestSearchCancelsInFlightQuery verifies that a keystroke arriving while a previous query is
// still in flight cancels that query's context before it completes, while a newer query started
// afterward keeps running uncanceled.
func TestSearchCancelsInFlightQuery(t *testing.T) {
	m := New(ctxUseCases{}, domain.LocaleENUS, theme.Default())
	m.tick = instantTick
	m.openSearch()
	m.searchField.SetValue("a")

	tick1 := m.restartSearch()
	screen, queryCmd1 := m.Update(tick1())
	m = screen.(*Model)
	if m.searchCancel == nil {
		t.Fatalf("expected the first query to register a cancel func")
	}

	// A second keystroke arrives before queryCmd1 ever runs: it must cancel queryCmd1's context.
	m.searchField.SetValue("ab")
	tick2 := m.restartSearch()

	msg1 := queryCmd1()
	res1, ok := msg1.(searchResultMsg)
	if !ok {
		t.Fatalf("expected a searchResultMsg, got %T", msg1)
	}
	if !errors.Is(res1.err, context.Canceled) {
		t.Fatalf("expected the first query's context to be canceled, got %v", res1.err)
	}

	screen, queryCmd2 := m.Update(tick2())
	m = screen.(*Model)
	if m.searchCancel == nil {
		t.Fatalf("expected the second query to register its own cancel func")
	}
	msg2 := queryCmd2()
	res2, ok := msg2.(searchResultMsg)
	if !ok {
		t.Fatalf("expected a searchResultMsg, got %T", msg2)
	}
	if res2.err != nil {
		t.Fatalf("expected the second query's context to still be live, got %v", res2.err)
	}
}
