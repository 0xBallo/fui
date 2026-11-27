package transactions

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/screentest"
	"github.com/0xballo/fui/internal/tui/theme"
)

// fakePending books or removes the transaction through the real service, so the test sees the
// screen reload what the port changed.
type fakePending struct {
	svc            *app.Service
	confirmed      []domain.TransactionID
	skipped        []domain.TransactionID
	confirmNotice  string
	confirmFailure error
}

func (f *fakePending) Confirm(ctx context.Context, id domain.TransactionID) (string, error) {
	if f.confirmFailure != nil {
		return "", f.confirmFailure
	}
	f.confirmed = append(f.confirmed, id)
	booked := string(domain.StatusBooked)
	plan, err := f.svc.PlanEditTx(ctx, app.EditTxRequest{ID: id, Status: &booked})
	if err != nil {
		return "", err
	}
	if _, err := f.svc.ApplyEditTx(ctx, plan); err != nil {
		return "", err
	}
	return f.confirmNotice, nil
}

func (f *fakePending) Skip(_ context.Context, id domain.TransactionID) (string, error) {
	f.skipped = append(f.skipped, id)
	return "skipped", nil
}

func pendingScreen(t *testing.T) (*Model, *fakePending) {
	t.Helper()
	fx := screentest.New(t)
	fx.Account(t, app.AddAccountRequest{Name: "Checking", Kind: "checking", Currency: "EUR", OpeningBalance: "1000", OpenedOn: "2026-01-01"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-10", Description: "Booked row", Date: "2026-06-09"})
	fx.Tx(t, app.AddTxRequest{Account: "Checking", Amount: "-30", Description: "Pending row", Date: "2026-06-10", Status: "pending"})
	fp := &fakePending{svc: fx.Service, confirmNotice: "booked it"}
	m := New(fx.Service, domain.LocaleENUS, theme.Default())
	m.SetPendingActions(fp)
	m.SetSize(100, 30)
	run(t, m, m.Init())
	return m, fp
}

func typeKey(scr screens.Screen, s string) (screens.Screen, tea.Cmd) {
	return scr.Update(tea.KeyPressMsg{Code: []rune(s)[0], Text: s})
}

func TestPendingConfirmBooksAndReloads(t *testing.T) {
	m, fp := pendingScreen(t) // the pending row is the newest, so it is selected first
	if m.selectedPending() == nil {
		t.Fatalf("expected the pending row selected: %+v", m.selectedTx())
	}
	var scr screens.Screen = m
	scr, _ = typeKey(scr, "c")
	if m.mode != modeConfirm || !strings.Contains(scr.View(), "Pending row") {
		t.Fatalf("expected a confirm dialog naming the row:\n%s", scr.View())
	}
	if len(fp.confirmed) != 0 {
		t.Fatal("nothing may be booked before the user confirms")
	}
	scr, cmd := typeKey(scr, "y")
	if cmd == nil {
		t.Fatal("confirming should run the action")
	}
	msg := cmd()
	done, ok := msg.(pendingDoneMsg)
	if !ok || done.notice != "booked it" {
		t.Fatalf("expected pendingDoneMsg with the notice, got %#v", msg)
	}
	scr, status := scr.Update(msg)
	if sm, ok := status().(screens.StatusMsg); !ok || sm.Text != "booked it" {
		t.Fatalf("expected the notice on the status bar, got %#v", status())
	}
	if len(fp.confirmed) != 1 || m.selectedPending() != nil || m.mode != modeList {
		t.Fatalf("expected the row booked and no longer pending (confirmed=%v mode=%v)", fp.confirmed, m.mode)
	}
	_ = scr
}

func TestPendingSkipAsksFirst(t *testing.T) {
	m, fp := pendingScreen(t)
	var scr screens.Screen = m
	scr, _ = typeKey(scr, "S")
	if m.mode != modeConfirm {
		t.Fatal("S on a pending row should ask for confirmation")
	}
	scr, _ = typeKey(scr, "n")
	if len(fp.skipped) != 0 || m.mode != modeList {
		t.Fatalf("declining must not skip (skipped=%v mode=%v)", fp.skipped, m.mode)
	}
	scr, _ = typeKey(scr, "S")
	scr, cmd := typeKey(scr, "y")
	scr = run(t, scr, cmd)
	if len(fp.skipped) != 1 {
		t.Fatalf("expected one skip, got %v", fp.skipped)
	}
	_ = scr
}

func TestPendingKeysIgnoreBookedRows(t *testing.T) {
	m, fp := pendingScreen(t)
	var scr screens.Screen = m
	scr, _ = typeKey(scr, "j") // move to the booked row
	if m.selectedPending() != nil {
		t.Fatal("expected the booked row selected")
	}
	scr, _ = typeKey(scr, "S")
	if m.mode != modeList || len(fp.skipped) != 0 {
		t.Fatalf("S must do nothing on a booked row (mode=%v)", m.mode)
	}
	scr, _ = typeKey(scr, "c")
	if m.mode != modeCategorize {
		t.Fatalf("c on a booked row keeps bulk categorize, got mode %v", m.mode)
	}
	_ = scr
}

func TestPendingKeysDisabledWithoutPort(t *testing.T) {
	m, _ := pendingScreen(t)
	m.SetPendingActions(nil)
	if got, want := len(m.KeyMap()), len(m.km.Bindings()); got != want {
		t.Fatalf("pending keys must stay out of the help without the port: %d vs %d", got, want)
	}
	var scr screens.Screen = m
	scr, _ = typeKey(scr, "S")
	if m.mode != modeList {
		t.Fatalf("S must do nothing without the port, mode=%v", m.mode)
	}
	_ = scr
	m.SetPendingActions(&fakePending{})
	if got, want := len(m.KeyMap()), len(m.km.Bindings())+2; got != want {
		t.Fatalf("help should list the two pending keys: %d vs %d", got, want)
	}
}

func TestPendingActionErrorSurfaces(t *testing.T) {
	m, fp := pendingScreen(t)
	fp.confirmFailure = domain.ErrNotFound
	var scr screens.Screen = m
	scr, _ = typeKey(scr, "c")
	_, cmd := typeKey(scr, "y")
	if em, ok := cmd().(screens.ErrorMsg); !ok || em.Err == nil {
		t.Fatal("expected the port error as an ErrorMsg")
	}
	if m.mode != modeList {
		t.Fatalf("the dialog must close on failure, mode=%v", m.mode)
	}
}
