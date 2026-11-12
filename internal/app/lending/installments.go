package lending

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// InterestCategory is the dedicated expense category of interest and fees.
const InterestCategory = "Loan interest"

// installmentDescription is the description of the pending transaction of a loan installment.
func installmentDescription(name string) string { return name + " installment" }

// PendingInstallment is one pending transaction to create for an installment.
type PendingInstallment struct {
	Tx           domain.Transaction
	LoanID       domain.LoanID
	ScheduleFrom domain.Date // ValidFrom of the schedule version the installment belongs to
	Due          domain.Date
}

// InstallmentsPlan is the plan to create the pending installment transactions.
type InstallmentsPlan struct {
	app.Plan
	Pending []PendingInstallment
}

// PlanInstallments plans one pending transaction on the debit account for every installment due
// through until (today when empty) that has none yet, for loan ref or for every loan when ref
// is empty. Running it twice creates nothing new.
func (s *Service) PlanInstallments(ctx context.Context, enabled module.Set, ref, until string) (InstallmentsPlan, error) {
	if err := require(enabled); err != nil {
		return InstallmentsPlan{}, err
	}
	end, err := s.date(until)
	if err != nil {
		return InstallmentsPlan{}, err
	}
	var out InstallmentsPlan
	err = s.store.Read(ctx, func(r app.Repos) error {
		infos, err := loanInfos(ctx, r)
		if err != nil {
			return err
		}
		if ref != "" {
			one, err := pickLoan(infos, ref)
			if err != nil {
				return err
			}
			infos = []Info{one}
		}
		for _, info := range infos {
			pend, err := s.pendingFor(ctx, r, info, end)
			if err != nil {
				return err
			}
			out.Pending = append(out.Pending, pend...)
		}
		for _, p := range out.Pending {
			f := map[string]any{
				"account_id": int64(p.Tx.AccountID), "date": p.Tx.Date.String(), "description": p.Tx.Description,
			}
			moneyFields(f, "amount", p.Tx.Amount)
			out.Changes = append(out.Changes, app.Change{
				Op: app.OpCreate, Entity: app.EntityTransaction, Fields: f,
				Summary: fmt.Sprintf("create pending installment %q due %s", p.Tx.Description, p.Due),
			})
		}
		out.Bulk = len(out.Pending) > 1
		return nil
	})
	return out, err
}

func (s *Service) pendingFor(ctx context.Context, r app.Repos, info Info, end domain.Date) ([]PendingInstallment, error) {
	starts, err := versionStarts(ctx, r, info.Loan)
	if err != nil {
		return nil, err
	}
	desc := installmentDescription(info.Loan.Name)
	var out []PendingInstallment
	for i, from := range starts {
		sched, err := r.Loans().ScheduleAt(ctx, info.Loan.ID, from)
		if err != nil {
			return nil, fmt.Errorf("schedule at %s: %w", from, err)
		}
		for _, inst := range sched.Installments {
			if inst.Due.Before(from) || inst.Due.After(end) ||
				(i+1 < len(starts) && !inst.Due.Before(starts[i+1])) {
				continue
			}
			if done, err := hasTransaction(ctx, r, info.Loan.ID, starts, inst.Due); err != nil || done {
				if err != nil {
					return nil, err
				}
				continue
			}
			tx, err := pendingTx(info, inst, desc)
			if err != nil || tx.Amount.IsZero() {
				if err != nil {
					return nil, err
				}
				continue
			}
			out = append(out, PendingInstallment{Tx: tx, LoanID: info.Loan.ID, ScheduleFrom: sched.ValidFrom, Due: inst.Due})
		}
	}
	return out, nil
}

// hasTransaction reports whether the installment due on due already has a transaction in any
// schedule version, so a new version does not create a second payment for a date already covered.
func hasTransaction(ctx context.Context, r app.Repos, loanID domain.LoanID, starts []domain.Date, due domain.Date) (bool, error) {
	for _, from := range starts {
		_, err := r.Loans().TransactionOfInstallment(ctx, loanID, from, due)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return false, fmt.Errorf("transaction of installment %s: %w", due, err)
		}
	}
	return false, nil
}

// versionStarts returns the ValidFrom dates of the schedule versions of loan, ascending.
func versionStarts(ctx context.Context, r app.Repos, loan domain.Loan) ([]domain.Date, error) {
	events, err := r.Loans().ListEvents(ctx, loan.ID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	starts := []domain.Date{loan.DisbursedOn}
	for _, e := range events {
		if e.EventDate.After(starts[len(starts)-1]) {
			starts = append(starts, e.EventDate)
		}
	}
	return starts, nil
}

// pendingTx builds the pending debit for inst; its Amount is zero when the installment is empty.
func pendingTx(info Info, inst domain.Installment, desc string) (domain.Transaction, error) {
	total, err := inst.Total()
	if err != nil || total.IsZero() {
		return domain.Transaction{}, err
	}
	neg, err := total.Neg()
	if err != nil {
		return domain.Transaction{}, err
	}
	return domain.NewTransaction(domain.Transaction{
		AccountID: info.Loan.DebitAccount, Date: inst.Due, DueDate: inst.Due, Amount: neg,
		Description: desc, Status: domain.StatusPending, Kind: domain.KindPayment,
	})
}

// ApplyInstallments creates the pending transactions and links them to their installments,
// after a backup when the plan is bulk. It returns how many were created.
func (s *Service) ApplyInstallments(ctx context.Context, p InstallmentsPlan) (int, error) {
	if len(p.Pending) == 0 {
		return 0, nil
	}
	err := s.write(ctx, p.Bulk, func(r app.Repos) error {
		for _, pi := range p.Pending {
			tx, err := r.Transactions().Insert(ctx, pi.Tx)
			if err != nil {
				return fmt.Errorf("create pending installment: %w", err)
			}
			if err := r.Loans().LinkInstallment(ctx, pi.LoanID, pi.ScheduleFrom, pi.Due, tx.ID); err != nil {
				return fmt.Errorf("link installment: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(p.Pending), nil
}

// ConfirmPlan is the plan to book a pending installment transaction as a principal transfer to
// the loan's liability account plus, when there is any, an interest and fees expense.
//
// When the amount actually debited differs from the schedule, Warnings says so and
// ProposedRate is the annual rate that would make the scheduled installment match it; the rate
// is only proposed, never applied (record a rate_change event to adopt it).
type ConfirmPlan struct {
	app.Plan
	Loan         Info
	Principal    domain.Transaction // the original transaction, turned into the transfer out leg
	In           domain.Transaction // transfer in leg on the liability account; zero when no principal
	Expense      domain.Transaction // interest and fees; zero when none
	Category     domain.Category    // ID zero until created on apply
	Warnings     []string
	ProposedRate domain.AnnualRate // zero when no proposal
}

// PlanConfirm splits the pending installment transaction txID into principal, and interest plus
// fees, following the schedule version in force at its due date.
func (s *Service) PlanConfirm(ctx context.Context, enabled module.Set, txID domain.TransactionID) (ConfirmPlan, error) {
	if err := require(enabled); err != nil {
		return ConfirmPlan{}, err
	}
	var out ConfirmPlan
	err := s.store.Read(ctx, func(r app.Repos) error {
		tx, err := r.Transactions().Get(ctx, txID)
		if err != nil {
			return fmt.Errorf("get transaction %d: %w", txID, err)
		}
		if tx.Status != domain.StatusPending || tx.Amount.Sign() >= 0 {
			return invalid("status", "transaction is not a pending payment")
		}
		ref, err := r.Loans().InstallmentOfTransaction(ctx, txID)
		if errors.Is(err, domain.ErrNotFound) {
			return invalid("transaction", "transaction is not a loan installment")
		}
		if err != nil {
			return fmt.Errorf("installment of transaction %d: %w", txID, err)
		}
		infos, err := loanInfos(ctx, r)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(infos, func(in Info) bool { return in.Loan.ID == ref.LoanID })
		if i < 0 {
			return fmt.Errorf("loan %d: %w", ref.LoanID, domain.ErrNotFound)
		}
		info := infos[i]
		sched, err := r.Loans().ScheduleAt(ctx, ref.LoanID, ref.ScheduleFrom)
		if err != nil {
			return fmt.Errorf("schedule at %s: %w", ref.ScheduleFrom, err)
		}
		j := slices.IndexFunc(sched.Installments, func(in domain.Installment) bool { return in.Due == ref.Due })
		if j < 0 {
			return invalid("due_date", fmt.Sprintf("no installment due on %s", ref.Due))
		}
		inst := sched.Installments[j]
		cat, err := interestCategory(ctx, r)
		if err != nil {
			return err
		}
		out, err = buildConfirm(info, tx, inst, sched.Rate, len(sched.Installments)-j, cat)
		return err
	})
	return out, err
}

func interestCategory(ctx context.Context, r app.Repos) (domain.Category, error) {
	cats, err := r.Categories().List(ctx)
	if err != nil {
		return domain.Category{}, fmt.Errorf("list categories: %w", err)
	}
	for _, c := range cats {
		if strings.EqualFold(c.Name, InterestCategory) {
			return c, nil
		}
	}
	return domain.NewCategory(InterestCategory, domain.CategoryExpense)
}

func buildConfirm(info Info, tx domain.Transaction, inst domain.Installment, rate domain.AnnualRate, termLeft int, cat domain.Category) (ConfirmPlan, error) {
	cur := tx.Amount.Currency()
	actual := -tx.Amount.Minor()
	principal := inst.Principal.Minor()
	rest := actual - principal // interest + fees
	p := ConfirmPlan{Loan: info, Category: cat}
	if rest < 0 {
		principal, rest = actual, 0
	}
	warn, err := mismatch(tx.Amount, inst)
	if err != nil {
		return ConfirmPlan{}, err
	}
	if warn != "" {
		p.Warnings = append(p.Warnings, warn)
		p.ProposedRate = proposeRate(info.Loan, inst, rate, termLeft, actual)
	}
	money := func(minor int64) (domain.Money, error) { return domain.NewMoney(minor, cur) }
	tx.Status = domain.StatusBooked
	if rest > 0 {
		exp := domain.Transaction{
			AccountID: tx.AccountID, Date: tx.Date, DueDate: tx.DueDate, Status: domain.StatusBooked,
			Kind: domain.KindInterest, CategoryID: cat.ID, Description: info.Loan.Name + " interest",
		}
		m, err := money(-rest)
		if err != nil {
			return ConfirmPlan{}, err
		}
		exp.Amount = m
		p.Expense = exp
	}
	if principal > 0 {
		out, err := money(-principal)
		if err != nil {
			return ConfirmPlan{}, err
		}
		in, err := money(principal)
		if err != nil {
			return ConfirmPlan{}, err
		}
		tx.Amount, tx.Kind = out, domain.KindTransfer
		p.In = domain.Transaction{
			AccountID: info.Account.ID, Date: tx.Date, Amount: in, Status: domain.StatusBooked,
			Kind: domain.KindTransfer, Description: info.Loan.Name + " principal",
		}
	} else {
		// No principal: the original transaction itself is the interest expense.
		tx.Amount, tx.Kind, tx.CategoryID = p.Expense.Amount, domain.KindInterest, cat.ID
		p.Expense = domain.Transaction{}
	}
	p.Principal = tx
	p.Changes = confirmChanges(p)
	return p, nil
}

// confirmChanges lists what applying p does, for rendering.
func confirmChanges(p ConfirmPlan) []app.Change {
	cur := p.Principal.Amount.Currency().Code()
	ch := []app.Change{{
		Op: app.OpUpdate, Entity: app.EntityTransaction,
		Summary: fmt.Sprintf("confirm installment %d of loan %q", p.Principal.ID, p.Loan.Loan.Name),
		Fields:  map[string]any{"id": int64(p.Principal.ID), "status": string(domain.StatusBooked)},
	}}
	if !p.In.Amount.Currency().IsZero() {
		ch = append(ch, app.Change{
			Op: app.OpCreate, Entity: app.EntityTransfer,
			Summary: fmt.Sprintf("transfer %s to loan %q", p.In.Amount, p.Loan.Loan.Name),
			Fields:  map[string]any{"to": p.Loan.Loan.Name, "amount": p.In.Amount.Minor(), "amount_currency": cur},
		})
	}
	if !p.Expense.Amount.Currency().IsZero() {
		ch = append(ch, app.Change{
			Op: app.OpCreate, Entity: app.EntityTransaction,
			Summary: fmt.Sprintf("book interest and fees %s in %q", p.Expense.Amount, p.Category.Name),
			Fields:  map[string]any{"amount": p.Expense.Amount.Minor(), "amount_currency": cur},
		})
	}
	return ch
}

// mismatch describes how the debited amount differs from the scheduled installment, "" if equal.
func mismatch(debited domain.Money, inst domain.Installment) (string, error) {
	scheduled, err := inst.Total()
	if err != nil {
		return "", err
	}
	if scheduled.Minor() == -debited.Minor() {
		return "", nil
	}
	return fmt.Sprintf("debited %s but the schedule expects %s", debited, scheduled), nil
}

// proposeRate finds the annual rate (basis points) whose first computed payment over the capital
// still owed before inst equals actual, or the closest one; 0 when no candidate exists. It
// is a proposal only.
func proposeRate(loan domain.Loan, inst domain.Installment, current domain.AnnualRate, termLeft int, actual int64) domain.AnnualRate {
	capital := inst.Remaining.Minor() + inst.Principal.Minor()
	if capital <= 0 {
		return 0
	}
	synth := loan
	m, err := domain.NewMoney(capital, loan.Principal.Currency())
	if err != nil {
		return 0
	}
	synth.Principal = m
	from, err := inst.Due.AddMonths(-1)
	if err != nil {
		return 0
	}
	term := termLeft
	first := func(bp int64) (int64, bool) {
		sch, err := domain.ComputeSchedule(synth, from, domain.AnnualRate(bp), term, 0)
		if err != nil || len(sch.Installments) == 0 {
			return 0, false
		}
		t, err := sch.Installments[0].Total()
		return t.Minor(), err == nil
	}
	lo, hi := int64(1), int64(5000)
	for lo < hi {
		mid := (lo + hi) / 2
		if v, ok := first(mid); !ok || v < actual {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if domain.AnnualRate(lo) == current {
		return 0
	}
	return domain.AnnualRate(lo)
}

// ApplyConfirm books the transaction of the plan: it turns it into the principal transfer leg
// (or the interest expense when the installment has no principal), stores the matching leg on
// the liability account and the interest and fees expense, creating the category when missing.
func (s *Service) ApplyConfirm(ctx context.Context, p ConfirmPlan) error {
	return s.write(ctx, false, func(r app.Repos) error {
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
			if _, err := r.Transactions().Insert(ctx, in); err != nil {
				return fmt.Errorf("create principal leg: %w", err)
			}
		}
		if err := r.Transactions().Update(ctx, out); err != nil {
			return fmt.Errorf("book installment: %w", err)
		}
		if !exp.Amount.Currency().IsZero() {
			exp.CategoryID = cat.ID
			if _, err := r.Transactions().Insert(ctx, exp); err != nil {
				return fmt.Errorf("create interest expense: %w", err)
			}
		}
		return nil
	})
}
