package lending

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// PendingKind tells what a confirmed pending transaction was.
type PendingKind string

// The kinds of pending transaction ConfirmPending can book.
const (
	PendingRecurrence  PendingKind = "recurrence"
	PendingInstallment PendingKind = "installment"
)

// PendingConfirmed is the outcome of ConfirmPending. Warnings and ProposedRate are only set for
// installments whose debited amount differs from the schedule.
type PendingConfirmed struct {
	Kind         PendingKind
	Warnings     []string
	ProposedRate domain.AnnualRate
}

// ConfirmWithin plans and applies the confirmation of the loan installment transaction txID
// inside the caller's repos, so it joins the caller's database transaction. The transaction may
// be pending or already booked (the importer books it first); it must be linked to an installment.
func ConfirmWithin(ctx context.Context, r app.Repos, txID domain.TransactionID) (ConfirmPlan, error) {
	tx, err := r.Transactions().Get(ctx, txID)
	if err != nil {
		return ConfirmPlan{}, fmt.Errorf("get transaction %d: %w", txID, err)
	}
	if tx.Amount.Sign() >= 0 {
		return ConfirmPlan{}, invalid("amount", "transaction is not a payment")
	}
	p, err := planConfirmTx(ctx, r, tx)
	if err != nil {
		return ConfirmPlan{}, err
	}
	if err := applyConfirm(ctx, r, p, nil); err != nil {
		return ConfirmPlan{}, err
	}
	return p, nil
}

// ConfirmPending books the pending transaction txID whatever generated it: a loan installment
// (needs the loans module) or a recurrence (needs the recurring module). Nothing is written on error.
func (s *Service) ConfirmPending(ctx context.Context, enabled module.Set, txID domain.TransactionID) (PendingConfirmed, error) {
	var linked, recurring bool
	err := s.store.Read(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, txID)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", txID, err)
		}
		_, err = r.Loans().InstallmentOfTransaction(ctx, txID)
		switch {
		case err == nil:
			linked = true
		case !errors.Is(err, domain.ErrNotFound):
			return fmt.Errorf("installment of transaction %d: %w", txID, err)
		}
		recurring = tx.RecurrenceID != 0
		return nil
	})
	if err != nil {
		return PendingConfirmed{}, err
	}
	switch {
	case linked:
		p, err := s.PlanConfirm(ctx, enabled, txID)
		if err != nil {
			return PendingConfirmed{}, err
		}
		if err := s.ApplyConfirm(ctx, p); err != nil {
			return PendingConfirmed{}, err
		}
		return PendingConfirmed{Kind: PendingInstallment, Warnings: p.Warnings, ProposedRate: p.ProposedRate}, nil
	case recurring && s.cfg.Recurring != nil:
		p, err := s.cfg.Recurring.PlanConfirm(ctx, enabled, txID)
		if err != nil {
			return PendingConfirmed{}, err
		}
		if err := s.cfg.Recurring.ApplyConfirm(ctx, p); err != nil {
			return PendingConfirmed{}, err
		}
		return PendingConfirmed{Kind: PendingRecurrence}, nil
	default:
		return PendingConfirmed{}, invalid("transaction", "transaction is not linked to a loan installment or a recurrence")
	}
}

// planConfirmTx builds the plan for tx, which must be a payment linked to an installment.
func planConfirmTx(ctx context.Context, r app.Repos, tx domain.Transaction) (ConfirmPlan, error) {
	ref, err := r.Loans().InstallmentOfTransaction(ctx, tx.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return ConfirmPlan{}, invalid("transaction", "transaction is not a loan installment")
	}
	if err != nil {
		return ConfirmPlan{}, fmt.Errorf("installment of transaction %d: %w", tx.ID, err)
	}
	infos, err := loanInfos(ctx, r)
	if err != nil {
		return ConfirmPlan{}, err
	}
	i := slices.IndexFunc(infos, func(in Info) bool { return in.Loan.ID == ref.LoanID })
	if i < 0 {
		return ConfirmPlan{}, fmt.Errorf("loan %d: %w", ref.LoanID, domain.ErrNotFound)
	}
	sched, err := r.Loans().ScheduleAt(ctx, ref.LoanID, ref.ScheduleFrom)
	if err != nil {
		return ConfirmPlan{}, fmt.Errorf("schedule at %s: %w", ref.ScheduleFrom, err)
	}
	j := slices.IndexFunc(sched.Installments, func(in domain.Installment) bool { return in.Due == ref.Due })
	if j < 0 {
		return ConfirmPlan{}, invalid("due_date", fmt.Sprintf("no installment due on %s", ref.Due))
	}
	cat, err := interestCategory(ctx, r)
	if err != nil {
		return ConfirmPlan{}, err
	}
	return buildConfirm(infos[i], tx, sched.Installments[j], sched.Rate, len(sched.Installments)-j, cat)
}

// applyConfirm stores the plan: the category when missing, the transfer legs, the booked
// original transaction and the interest expense. A non-nil clock journals the write for undo;
// the importer passes nil because its confirmation is undone with the import batch.
func applyConfirm(ctx context.Context, r app.Repos, p ConfirmPlan, clock app.Clock) error {
	if clock == nil {
		return storeConfirm(ctx, r, p, nil)
	}
	j, err := app.BeginJournal(ctx, r, p.Principal.ID)
	if err != nil {
		return err
	}
	if err := storeConfirm(ctx, r, p, j); err != nil {
		return err
	}
	return j.Commit(ctx, r, clock, app.OpKindLoanConfirm)
}

// storeConfirm writes the plan, tracking the rows it creates in j when j is not nil.
func storeConfirm(ctx context.Context, r app.Repos, p ConfirmPlan, j *app.Journal) error {
	cat := p.Category
	if cat.ID == 0 {
		var err error
		if cat, err = r.Categories().Insert(ctx, cat); err != nil {
			return fmt.Errorf("create category: %w", err)
		}
	}
	out, in, exp := p.Principal, p.In, p.Expense
	if out.CategoryID == 0 && out.Kind == domain.KindInterest {
		out.CategoryID = cat.ID
	}
	if !in.Amount.Currency().IsZero() {
		g, err := r.Transfers().Insert(ctx, domain.TransferGroup{})
		if err != nil {
			return fmt.Errorf("create transfer: %w", err)
		}
		out.TransferGroupID, in.TransferGroupID = g.ID, g.ID
		leg, err := r.Transactions().Insert(ctx, in)
		if err != nil {
			return fmt.Errorf("create principal leg: %w", err)
		}
		if j != nil {
			j.Track(leg.ID)
			j.TrackGroup(g.ID)
		}
	}
	if err := r.Transactions().Update(ctx, out); err != nil {
		return fmt.Errorf("book installment: %w", err)
	}
	if !exp.Amount.Currency().IsZero() {
		exp.CategoryID = cat.ID
		row, err := r.Transactions().Insert(ctx, exp)
		if err != nil {
			return fmt.Errorf("create interest expense: %w", err)
		}
		if j != nil {
			j.Track(row.ID)
		}
	}
	return nil
}
