package accounts

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
)

// reconcilePlannedMsg carries a (re)computed plan and the candidate transactions to list.
type reconcilePlannedMsg struct {
	plan  app.ReconcilePlan
	cands []domain.Transaction
}

// reconcileSavedMsg reports that the current ticks were persisted without closing.
type reconcileSavedMsg struct{ notice string }

type reconcileAppliedMsg struct{ loaded loadedMsg }

func (m *Model) setReconcilePlan(msg reconcilePlannedMsg) {
	m.reconcilePlan = msg.plan
	m.reconcileCands = msg.cands
	m.reconcileCursor = min(m.reconcileCursor, max(0, len(msg.cands)-1))
	m.mode = modeReconcilePreview
}

// planReconcileCmd plans the reconciliation with the given tick changes. The candidates are the
// unticked transactions of the plan plus the ticked ones; ticked ones are not in the plan, so they
// are taken from known or, for ticks stored by an earlier session, looked up in the account.
func (m *Model) planReconcileCmd(tick, untick []domain.TransactionID) tea.Cmd {
	uc := m.uc
	req := app.ReconcileRequest{
		Account: m.reconcileAccount, StatementBalance: m.reconcileBalance, Date: m.reconcileDate,
		Tick: tick, Untick: untick,
	}
	known := slices.Clone(m.reconcileCands)
	return func() tea.Msg {
		ctx := context.Background()
		plan, err := uc.PlanReconcile(ctx, module.Default(), req)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		cands, err := reconcileCandidates(ctx, uc, plan, known)
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reconcilePlannedMsg{plan: plan, cands: cands}
	}
}

func reconcileCandidates(
	ctx context.Context, uc UseCases, plan app.ReconcilePlan, known []domain.Transaction,
) ([]domain.Transaction, error) {
	byID := make(map[domain.TransactionID]domain.Transaction, len(known)+len(plan.Unreconciled))
	for _, tx := range known {
		byID[tx.ID] = tx
	}
	for _, tx := range plan.Unreconciled {
		byID[tx.ID] = tx
	}
	if slices.ContainsFunc(plan.Ticked, func(id domain.TransactionID) bool { _, ok := byID[id]; return !ok }) {
		if err := loadAccountTx(ctx, uc, plan, byID); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Transaction, 0, len(plan.Unreconciled)+len(plan.Ticked))
	out = append(out, plan.Unreconciled...)
	for _, id := range plan.Ticked {
		if tx, ok := byID[id]; ok && tx.Status == domain.StatusBooked && !tx.Date.After(plan.Date) {
			out = append(out, tx)
		}
	}
	slices.SortFunc(out, func(a, b domain.Transaction) int {
		if c := a.Date.Cmp(b.Date); c != 0 {
			return c
		}
		return int(a.ID - b.ID)
	})
	return slices.CompactFunc(out, func(a, b domain.Transaction) bool { return a.ID == b.ID }), nil
}

func loadAccountTx(
	ctx context.Context, uc UseCases, plan app.ReconcilePlan, byID map[domain.TransactionID]domain.Transaction,
) error {
	var after *app.TxCursor
	for {
		page, err := uc.ListTx(ctx, app.ListTxRequest{
			Accounts: []string{plan.Account.Name}, To: plan.Date.String(), After: after,
		})
		if err != nil {
			return fmt.Errorf("list account transactions: %w", err)
		}
		for _, v := range page.Items {
			byID[v.Transaction.ID] = v.Transaction
		}
		if page.Next == nil {
			return nil
		}
		after = page.Next
	}
}

func (m *Model) handleReconcilePreviewKey(msg tea.KeyPressMsg) (screens.Screen, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.mode = m.reconcileBackMode
	case key.Matches(msg, m.km.Up):
		m.reconcileCursor = max(0, m.reconcileCursor-1)
	case key.Matches(msg, m.km.Down):
		m.reconcileCursor = min(max(0, len(m.reconcileCands)-1), m.reconcileCursor+1)
	case key.Matches(msg, m.km.Tick):
		return m, m.toggleTick()
	case msg.String() == "enter":
		return m, m.confirmReconcile()
	}
	return m, nil
}

// toggleTick re-plans with the cursor transaction flipped. The request carries the whole tick
// state (ticked and unticked candidates) so toggles not yet persisted survive the re-plan.
func (m *Model) toggleTick() tea.Cmd {
	if m.reconcileCursor >= len(m.reconcileCands) {
		return nil
	}
	m.reconcileNotice = ""
	target := m.reconcileCands[m.reconcileCursor].ID
	ticked := slices.Clone(m.reconcilePlan.Ticked)
	if i := slices.Index(ticked, target); i >= 0 {
		ticked = slices.Delete(ticked, i, i+1)
	} else {
		ticked = append(ticked, target)
	}
	var untick []domain.TransactionID
	for _, tx := range m.reconcileCands {
		if !slices.Contains(ticked, tx.ID) {
			untick = append(untick, tx.ID)
		}
	}
	return m.planReconcileCmd(ticked, untick)
}

// confirmReconcile closes when the difference is zero; otherwise it saves the ticks and tells how
// far the statement is.
func (m *Model) confirmReconcile() tea.Cmd {
	plan := m.reconcilePlan
	if plan.Closing {
		return m.submitReconcileApply()
	}
	notice := fmt.Sprintf("difference %s: tick the remaining transactions", domain.FormatAmount(plan.Difference, m.loc))
	if !plan.Ticking {
		m.reconcileNotice = notice
		return nil
	}
	uc := m.uc
	return func() tea.Msg {
		if err := uc.ApplyReconcile(context.Background(), module.Default(), plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return reconcileSavedMsg{notice: notice + " (ticks saved)"}
	}
}

func (m *Model) submitReconcileApply() tea.Cmd {
	plan := m.reconcilePlan
	uc := m.uc
	return func() tea.Msg {
		ctx := context.Background()
		if err := uc.ApplyReconcile(ctx, module.Default(), plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		msg := m.fetch(ctx)
		loaded, ok := msg.(loadedMsg)
		if !ok {
			return msg
		}
		return reconcileAppliedMsg{loaded: loaded}
	}
}

func (m *Model) viewReconcilePreview() string {
	p := m.reconcilePlan
	out := m.th.Title.Render("Reconcile preview") + "\n"
	out += m.th.Base.Render("Account: " + p.Account.Name)
	out += "\n" + m.th.Base.Render("Date: "+p.Date.String()+"   Statement: "+domain.FormatAmount(p.StatementBalance, m.loc))
	diffLine := "Difference: " + domain.FormatAmount(p.Difference, m.loc)
	if p.Closing {
		out += "\n" + m.th.StatusBar.Render(diffLine)
		out += "\n" + m.th.Muted.Render("enter close reconciliation · space tick · j/k move · esc back")
	} else {
		out += "\n" + m.th.Warning.Render(diffLine)
		out += "\n" + m.th.Muted.Render("space tick · enter save ticks · j/k move · esc back")
	}
	if len(m.reconcileCands) > 0 {
		out += "\n\n" + m.th.Header.Render("Transactions up to date:")
		for i, tx := range m.reconcileCands {
			box, cursor := "[ ]", "  "
			if slices.Contains(p.Ticked, tx.ID) {
				box = "[x]"
			}
			if i == m.reconcileCursor {
				cursor = "> "
			}
			line := cursor + box + " " + tx.Date.Format(m.loc) + "  " + tx.Description + "  " +
				strings.TrimSpace(domain.FormatAmount(tx.Amount, m.loc))
			if i == m.reconcileCursor {
				out += "\n" + m.th.Title.Render(line)
			} else {
				out += "\n" + m.th.Base.Render(line)
			}
		}
	}
	if m.reconcileNotice != "" {
		out += "\n\n" + m.th.Warning.Render(m.reconcileNotice)
	}
	return out
}
