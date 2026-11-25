package app

import (
	"context"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

const entityReconciliation = "reconciliation"

// ReconcileRequest asks to reconcile one account against a statement balance at a date.
// Date follows the same parsing rules as other app date inputs: empty means today.
type ReconcileRequest struct {
	Account          string
	StatementBalance string
	Date             string
	// Tick and Untick add or remove transactions of the account from the ticked set. When either
	// is set, or ticks are already stored, the reconciliation is incremental: only ticked
	// transactions count towards the balance and it closes once the difference is zero.
	Tick, Untick []domain.TransactionID
}

// ReconcilePlan is the reconciliation preview and, when Difference is zero, the close action.
type ReconcilePlan struct {
	Plan
	Account          domain.Account
	Date             domain.Date
	StatementBalance domain.Money
	Difference       domain.Money
	Unreconciled     []domain.Transaction
	ReconciledIDs    []domain.TransactionID
	// Ticking marks an incremental plan: ApplyReconcile stores Ticked and removes Untick even
	// when the difference is not zero, and closes only when Closing is set.
	Ticking bool
	Ticked  []domain.TransactionID
	Untick  []domain.TransactionID
	// Closing is true when the difference is zero and applying the plan closes the reconciliation.
	Closing bool
}

// PlanReconcile computes the statement difference and the booked transactions up to Date that are
// still unreconciled. When Difference is zero, it also plans closing reconciliation.
func (s *Service) PlanReconcile(ctx context.Context, enabled module.Set, req ReconcileRequest) (ReconcilePlan, error) {
	if err := WrapModuleError(enabled.Require(module.Core)); err != nil {
		return ReconcilePlan{}, err
	}
	var out ReconcilePlan
	err := s.store.Read(ctx, func(r Repos) error {
		acct, asOf, statementBalance, txs, err := s.reconcileInputs(ctx, r, req)
		if err != nil {
			return err
		}
		persisted, err := r.Reconciliations().Ticks(ctx, acct.ID)
		if err != nil {
			return fmt.Errorf("list reconcile ticks: %w", err)
		}
		if len(persisted) == 0 && len(req.Tick) == 0 && len(req.Untick) == 0 {
			out, err = planBulkReconcile(acct, asOf, statementBalance, txs)
			return err
		}
		if err := checkTicksBelong(ctx, r, acct, req.Tick, req.Untick); err != nil {
			return err
		}
		out, err = planTickedReconcile(acct, asOf, statementBalance, txs, mergeTicks(persisted, req))
		out.Untick = slices.Clone(req.Untick)
		return err
	})
	return out, err
}

func planBulkReconcile(
	acct domain.Account,
	asOf domain.Date,
	statementBalance domain.Money,
	txs []domain.Transaction,
) (ReconcilePlan, error) {
	difference, unreconciledIDs, err := domain.Reconcile(statementBalance, acct.OpeningBalance, txs, asOf)
	if err != nil {
		return ReconcilePlan{}, fmt.Errorf("reconcile account %q: %w", acct.Name, err)
	}
	out := ReconcilePlan{
		Account:          acct,
		Date:             asOf,
		StatementBalance: statementBalance,
		Difference:       difference,
		Unreconciled:     collectTransactions(indexTransactions(txs), unreconciledIDs),
	}
	if !difference.IsZero() {
		return out, nil
	}
	ids, err := domain.CloseReconciliation(statementBalance, acct.OpeningBalance, txs, asOf)
	if err != nil {
		return ReconcilePlan{}, fmt.Errorf("close reconciliation: %w", err)
	}
	out.Closing = true
	out.ReconciledIDs = ids
	out.Bulk = len(ids) > 0
	out.Changes = reconcileChanges(acct, asOf, statementBalance, ids)
	return out, nil
}

func planTickedReconcile(
	acct domain.Account,
	asOf domain.Date,
	statementBalance domain.Money,
	txs []domain.Transaction,
	ticked []domain.TransactionID,
) (ReconcilePlan, error) {
	difference, untickedIDs, err := domain.ReconcileTicked(statementBalance, acct.OpeningBalance, txs, ticked, asOf)
	if err != nil {
		return ReconcilePlan{}, fmt.Errorf("reconcile account %q: %w", acct.Name, err)
	}
	out := ReconcilePlan{
		Account:          acct,
		Date:             asOf,
		StatementBalance: statementBalance,
		Difference:       difference,
		Unreconciled:     collectTransactions(indexTransactions(txs), untickedIDs),
		Ticking:          true,
		Ticked:           ticked,
	}
	if !difference.IsZero() {
		return out, nil
	}
	// Ticks of transactions outside the booked window stay stored but are not reconciled.
	for _, tx := range txs {
		if tx.Status == domain.StatusBooked && !tx.Date.After(asOf) && slices.Contains(ticked, tx.ID) {
			out.ReconciledIDs = append(out.ReconciledIDs, tx.ID)
		}
	}
	slices.Sort(out.ReconciledIDs)
	out.Closing = true
	out.Bulk = len(out.ReconciledIDs) > 0
	out.Changes = reconcileChanges(acct, asOf, statementBalance, out.ReconciledIDs)
	return out, nil
}

// mergeTicks returns the sorted, deduplicated persisted+Tick set minus Untick.
func mergeTicks(persisted []domain.TransactionID, req ReconcileRequest) []domain.TransactionID {
	set := make(map[domain.TransactionID]struct{}, len(persisted)+len(req.Tick))
	for _, id := range persisted {
		set[id] = struct{}{}
	}
	for _, id := range req.Tick {
		set[id] = struct{}{}
	}
	for _, id := range req.Untick {
		delete(set, id)
	}
	out := make([]domain.TransactionID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func checkTicksBelong(ctx context.Context, r Repos, acct domain.Account, groups ...[]domain.TransactionID) error {
	for _, ids := range groups {
		for _, id := range ids {
			tx, err := r.Transactions().Get(ctx, id)
			if err != nil {
				return fmt.Errorf("get transaction %d: %w", id, err)
			}
			if tx.AccountID != acct.ID {
				return newError(domain.ErrInvalidValue, map[string]any{
					"field": "transaction",
					"id":    int64(id),
				}, "transaction %d does not belong to account %q", id, acct.Name)
			}
		}
	}
	return nil
}

// ApplyReconcile marks planned transactions as reconciled and records the reconciliation close.
func (s *Service) ApplyReconcile(ctx context.Context, enabled module.Set, p ReconcilePlan) error {
	if err := WrapModuleError(enabled.Require(module.Core)); err != nil {
		return err
	}
	if (!p.Ticking || p.Closing) && !p.Difference.IsZero() {
		return newError(domain.ErrInvalidValue, map[string]any{
			"field":      "difference",
			"difference": p.Difference.Minor(),
		}, "cannot close reconciliation when difference is %s", p.Difference)
	}
	return s.apply(ctx, p.Bulk, func(r Repos) error {
		if p.Ticking {
			if err := storeTicks(ctx, r, p); err != nil {
				return err
			}
			if !p.Closing {
				return nil
			}
		}
		return s.closeReconciliation(ctx, r, p)
	})
}

func storeTicks(ctx context.Context, r Repos, p ReconcilePlan) error {
	if err := r.Reconciliations().SetTicks(ctx, p.Account.ID, p.Ticked, true); err != nil {
		return fmt.Errorf("store reconcile ticks: %w", err)
	}
	if err := r.Reconciliations().SetTicks(ctx, p.Account.ID, p.Untick, false); err != nil {
		return fmt.Errorf("clear reconcile ticks: %w", err)
	}
	return nil
}

func (s *Service) closeReconciliation(ctx context.Context, r Repos, p ReconcilePlan) error {
	for _, id := range p.ReconciledIDs {
		tx, err := r.Transactions().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", id, err)
		}
		tx.Status = domain.StatusReconciled
		updated, err := domain.NewTransaction(tx)
		if err != nil {
			return fmt.Errorf("transaction %d: %w", id, err)
		}
		if err := r.Transactions().Update(ctx, updated); err != nil {
			return fmt.Errorf("update transaction %d: %w", id, err)
		}
	}
	rec, err := r.Reconciliations().Open(ctx, Reconciliation{
		AccountID:        p.Account.ID,
		Date:             p.Date,
		StatementBalance: p.StatementBalance,
	})
	if err != nil {
		return fmt.Errorf("open reconciliation: %w", err)
	}
	if err := r.Reconciliations().Close(ctx, rec.AccountID, rec.Date, s.clock.Now().UTC()); err != nil {
		return fmt.Errorf("close reconciliation: %w", err)
	}
	if p.Ticking {
		if err := r.Reconciliations().SetTicks(ctx, p.Account.ID, p.ReconciledIDs, false); err != nil {
			return fmt.Errorf("clear reconcile ticks: %w", err)
		}
	}
	return nil
}

func (s *Service) reconcileInputs(
	ctx context.Context,
	r Repos,
	req ReconcileRequest,
) (domain.Account, domain.Date, domain.Money, []domain.Transaction, error) {
	accts, err := r.Accounts().List(ctx)
	if err != nil {
		return domain.Account{}, domain.Date{}, domain.Money{}, nil, fmt.Errorf("list accounts: %w", err)
	}
	acct, err := pickAccount(accts, req.Account)
	if err != nil {
		return domain.Account{}, domain.Date{}, domain.Money{}, nil, err
	}
	asOf, err := s.date(req.Date)
	if err != nil {
		return domain.Account{}, domain.Date{}, domain.Money{}, nil, err
	}
	statementBalance, err := s.amount(req.StatementBalance, acct.Currency)
	if err != nil {
		return domain.Account{}, domain.Date{}, domain.Money{}, nil, err
	}
	filter, err := domain.NewTxFilter(domain.TxFilterSpec{AccountIDs: []domain.AccountID{acct.ID}, To: asOf})
	if err != nil {
		return domain.Account{}, domain.Date{}, domain.Money{}, nil, fmt.Errorf("transaction filter: %w", err)
	}
	txs, err := r.Transactions().List(ctx, filter, nil, 0)
	if err != nil {
		return domain.Account{}, domain.Date{}, domain.Money{}, nil, fmt.Errorf("list account transactions: %w", err)
	}
	return acct, asOf, statementBalance, txs, nil
}

func indexTransactions(txs []domain.Transaction) map[domain.TransactionID]domain.Transaction {
	out := make(map[domain.TransactionID]domain.Transaction, len(txs))
	for _, tx := range txs {
		out[tx.ID] = tx
	}
	return out
}

func collectTransactions(
	txByID map[domain.TransactionID]domain.Transaction,
	ids []domain.TransactionID,
) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(ids))
	for _, id := range ids {
		if tx, ok := txByID[id]; ok {
			out = append(out, tx)
		}
	}
	return out
}

func reconcileChanges(
	acct domain.Account,
	asOf domain.Date,
	statementBalance domain.Money,
	ids []domain.TransactionID,
) []Change {
	changes := make([]Change, 0, len(ids)+1)
	for _, id := range ids {
		changes = append(changes, Change{
			Op:      OpUpdate,
			Entity:  EntityTransaction,
			Summary: fmt.Sprintf("mark transaction %d as reconciled", id),
			Fields: map[string]any{
				"id":     int64(id),
				"status": string(domain.StatusReconciled),
			},
		})
	}
	changes = append(changes, Change{
		Op:      OpCreate,
		Entity:  entityReconciliation,
		Summary: fmt.Sprintf("close reconciliation for account %s at %s", acct.Name, asOf),
		Fields: map[string]any{
			"account_id":        int64(acct.ID),
			"date":              asOf.String(),
			"statement_balance": statementBalance.Minor(),
			"currency":          statementBalance.Currency().Code(),
		},
	})
	return changes
}
