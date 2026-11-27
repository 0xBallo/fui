package transactions

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/screens"
)

// Purposes of the confirm dialog for pending rows, continuing the sequence declared in form.go.
const (
	confirmPendingConfirm = confirmCategorizeReconciled + 1 + iota
	confirmPendingSkip
)

// SetPendingActions enables the confirm and skip keys on pending rows.
func (m *Model) SetPendingActions(p PendingActions) { m.pending = p }

// selectedPending returns the highlighted row when it is pending and the port is wired.
func (m *Model) selectedPending() *domain.Transaction {
	if m.pending == nil {
		return nil
	}
	row := m.selectedTx()
	if row == nil || row.Transaction.Status != domain.StatusPending {
		return nil
	}
	return &row.Transaction
}

// handlePendingKey arms the confirm dialog for "c" or "S" on a pending row, reporting whether msg
// was consumed; on any other row "c" falls through to bulk categorize.
func (m *Model) handlePendingKey(msg tea.KeyPressMsg) bool {
	confirm, skip := key.Matches(msg, m.km.ConfirmPending), key.Matches(msg, m.km.SkipPending)
	if !confirm && !skip {
		return false
	}
	tx := m.selectedPending()
	if tx == nil {
		return false
	}
	m.pendingTarget = tx.ID
	if skip {
		m.confirmPurpose = confirmPendingSkip
		m.confirm.Show("Skip pending transaction", fmt.Sprintf("Discard pending %q? The recurrence is kept.", tx.Description))
	} else {
		m.confirmPurpose = confirmPendingConfirm
		m.confirm.Show("Confirm pending transaction",
			fmt.Sprintf("Book %q for %s?", tx.Description, domain.FormatAmount(tx.Amount, m.loc)))
	}
	m.mode = modeConfirm
	return true
}

// pendingDoneMsg carries the notice of a resolved pending row and the reloaded first page.
type pendingDoneMsg struct {
	notice string
	loaded loadedMsg
}

// ScreenID routes the result to the transactions tab.
func (pendingDoneMsg) ScreenID() screens.ID { return screens.TabTransactions }

func (m *Model) submitPending(purpose confirmPurpose) tea.Cmd {
	m.mode = modeList
	id, uc, req, pending := m.pendingTarget, m.uc, m.req, m.pending
	return func() tea.Msg {
		ctx := context.Background()
		act := pending.Confirm
		if purpose == confirmPendingSkip {
			act = pending.Skip
		}
		notice, err := act(ctx, id)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		switch loaded := reload(ctx, uc, req, false).(type) {
		case loadedMsg:
			return pendingDoneMsg{notice: notice, loaded: loaded}
		default:
			return loaded
		}
	}
}
