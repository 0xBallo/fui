package importing

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// LoanPlanRow is one loan schedule version to import, with the loan it belongs to. Rows are
// planned only; the apply step writes them.
type LoanPlanRow struct {
	Line       int // line of the schedule record
	AccountRef string
	Include    bool
	// Module is set when the loans module is not enabled: the row is imported only if
	// ApplyOptions.EnableModules names it, and skipped with a warning otherwise.
	Module module.ID
	// New is set when the loan does not exist yet and the import creates it from Loan. For an
	// existing loan Loan is the stored one and Version.LoanID is its ID.
	New  bool
	Loan domain.Loan
	// DebitRef names the debit account of a new loan: a stream account ref or an account name.
	DebitRef string
	Version  lending.ImportedVersion
	// Unchanged is set when the loan already stores this very version, so re-importing it is a no-op.
	Unchanged bool
}

// loanGroup is a schedule record with the installment records that follow it.
type loanGroup struct {
	rec   *fuiproto.ScheduleRecord
	line  int
	insts []*fuiproto.InstallmentRecord
}

// addLoans plans the loan schedule versions of the stream. A group that cannot be imported is
// skipped with a warning: it never fails the batch.
func (b *builder) addLoans(ctx context.Context) error {
	var groups []*loanGroup
	last := map[string]*loanGroup{}
	for _, rec := range b.st.result.Records {
		switch {
		case rec.Schedule != nil:
			g := &loanGroup{rec: rec.Schedule, line: b.st.sched[rec.Schedule]}
			groups = append(groups, g)
			last[rec.Schedule.Account] = g
		case rec.Installment != nil:
			if g := last[rec.Installment.Account]; g != nil {
				g.insts = append(g.insts, rec.Installment)
				continue
			}
			b.warn(b.st.inst[rec.Installment], "installment skipped: no schedule for account %q", rec.Installment.Account)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	loans, err := b.r.Loans().List(ctx)
	if err != nil {
		return fmt.Errorf("list loans: %w", err)
	}
	created := map[string]domain.Loan{} // new loans of this stream by account ref
	for _, g := range groups {
		row, ok, err := b.loanRow(ctx, g, loans, created)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if !b.req.Modules.Has(module.Loans) {
			row.Module = module.Loans
			b.noteModuleRecord(module.Loans, "schedule", g.line)
			b.warn(g.line, "schedule needs module %s: enable it or the record is skipped", module.Loans)
			for _, in := range g.insts {
				b.noteModuleRecord(module.Loans, "installment", b.st.inst[in])
				b.warn(b.st.inst[in], "installment needs module %s: enable it or the record is skipped", module.Loans)
			}
		}
		if row.New {
			created[row.AccountRef] = row.Loan
		}
		b.plan.Loans = append(b.plan.Loans, row)
	}
	return nil
}

// loanRow builds the plan row of one group; ok is false when the group was skipped.
func (b *builder) loanRow(ctx context.Context, g *loanGroup, loans []domain.Loan, created map[string]domain.Loan) (LoanPlanRow, bool, error) {
	ref := g.rec.Account
	info, name, ok, err := b.loanAccount(ctx, g)
	if err != nil || !ok {
		return LoanPlanRow{}, false, err
	}
	v, ok := b.loanVersion(g, info.cur)
	if !ok {
		return LoanPlanRow{}, false, nil
	}
	row := LoanPlanRow{Line: g.line, AccountRef: ref, Include: true, Version: v}

	if loan, ok := created[ref]; ok {
		row.Loan = loan
		return row, true, nil
	}
	if info.id > 0 {
		i := slices.IndexFunc(loans, func(l domain.Loan) bool { return l.LiabilityAccountID == info.id })
		if i >= 0 {
			row.Loan, row.Version.LoanID = loans[i], loans[i].ID
			same, err := b.versionStored(ctx, row.Loan, v)
			row.Unchanged = same
			row.Include = !same
			return row, true, err
		}
	}
	return b.newLoanRow(row, g, info, name, loans)
}

// loanAccount resolves the schedule's account to a loan-kind account, returning its name.
func (b *builder) loanAccount(ctx context.Context, g *loanGroup) (*acctInfo, string, bool, error) {
	ref := g.rec.Account
	info := b.accts[ref]
	if info == nil || !info.resolved() {
		b.warn(g.line, "schedule skipped: account %q is unresolved", ref)
		return nil, "", false, nil
	}
	kind, name := domain.AccountKind(info.rec.Kind), strings.TrimSpace(info.rec.Name)
	if info.mapped {
		a, err := b.r.Accounts().Get(ctx, info.id)
		if err != nil {
			return nil, "", false, fmt.Errorf("get account %q: %w", ref, err)
		}
		kind, name = a.Kind, a.Name
	}
	if kind != domain.AccountLoan {
		b.warn(g.line, "schedule skipped: account %q is not a loan account", ref)
		return nil, "", false, nil
	}
	return info, name, true, nil
}

// loanVersion converts the group to the bank's schedule version, in the account currency.
func (b *builder) loanVersion(g *loanGroup, cur domain.Currency) (lending.ImportedVersion, bool) {
	from, err := domain.ParseDate(g.rec.ValidFrom)
	if err != nil {
		b.warn(g.line, "schedule skipped: valid_from: %v", err)
		return lending.ImportedVersion{}, false
	}
	rate, err := domain.NewAnnualRate(g.rec.Rate)
	if err != nil {
		b.warn(g.line, "schedule skipped: rate: %v", err)
		return lending.ImportedVersion{}, false
	}
	if len(g.insts) == 0 {
		b.warn(g.line, "schedule skipped: no installments")
		return lending.ImportedVersion{}, false
	}
	v := lending.ImportedVersion{ValidFrom: from, Rate: rate, RateType: domain.RateType(g.rec.RateType)}
	for _, in := range g.insts {
		inst, err := installment(*in, cur)
		if err != nil {
			b.warn(b.st.inst[in], "schedule skipped: installment: %v", err)
			return lending.ImportedVersion{}, false
		}
		v.Installments = append(v.Installments, inst)
	}
	return v, true
}

func installment(in fuiproto.InstallmentRecord, cur domain.Currency) (domain.Installment, error) {
	due, err := domain.ParseDate(in.DueDate)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("due_date: %w", err)
	}
	out := domain.Installment{Due: due}
	for _, f := range []struct {
		name string
		v    int64
		dst  *domain.Money
	}{
		{"principal", in.Principal, &out.Principal},
		{"interest", in.Interest, &out.Interest},
		{"fees", in.Fees, &out.Fees},
		{"remaining", in.Remaining, &out.Remaining},
	} {
		if f.v < 0 {
			return domain.Installment{}, fmt.Errorf("%s must not be negative", f.name)
		}
		if *f.dst, err = domain.NewMoney(f.v, cur); err != nil {
			return domain.Installment{}, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return out, nil
}

// versionStored reports whether the loan already has a version at v.ValidFrom with the same
// rate and installments, so that re-importing the same file changes nothing.
func (b *builder) versionStored(ctx context.Context, loan domain.Loan, v lending.ImportedVersion) (bool, error) {
	s, err := b.r.Loans().ScheduleAt(ctx, loan.ID, v.ValidFrom)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("schedule of loan %q at %s: %w", loan.Name, v.ValidFrom, err)
	}
	return s.ValidFrom == v.ValidFrom && s.Rate == v.Rate && slices.Equal(s.Installments, v.Installments), nil
}

// newLoanRow completes row for an account that has no loan: the schedule record must carry the
// loan metadata, else the whole group is skipped.
func (b *builder) newLoanRow(row LoanPlanRow, g *loanGroup, info *acctInfo, name string, loans []domain.Loan) (LoanPlanRow, bool, error) {
	rec := g.rec
	var missing []string
	for _, m := range []struct {
		name string
		ok   bool
	}{
		{"debit_account", rec.DebitAccount != ""},
		{"kind", rec.Kind != ""},
		{"principal", rec.Principal > 0},
		{"disbursed_on", rec.DisbursedOn != ""},
		{"term_months", rec.TermMonths > 0},
	} {
		if !m.ok {
			missing = append(missing, m.name)
		}
	}
	if len(missing) > 0 {
		b.warn(g.line, "loan not created: schedule lacks %s", strings.Join(missing, ", "))
		return LoanPlanRow{}, false, nil
	}
	disbursed, err := domain.ParseDate(rec.DisbursedOn)
	if err != nil {
		b.warn(g.line, "loan not created: disbursed_on: %v", err)
		return LoanPlanRow{}, false, nil
	}
	debit, debitRef, ok := b.debitAccount(rec.DebitAccount)
	if !ok {
		b.warn(g.line, "loan not created: debit account %q not found", rec.DebitAccount)
		return LoanPlanRow{}, false, nil
	}
	principal, err := domain.NewMoney(rec.Principal, info.cur)
	if err != nil {
		b.warn(g.line, "loan not created: principal: %v", err)
		return LoanPlanRow{}, false, nil
	}
	loan, err := domain.NewLoan(domain.Loan{
		Name: name, LiabilityAccountID: info.id, Kind: domain.LoanKind(rec.Kind), Principal: principal,
		DisbursedOn: disbursed, TermMonths: rec.TermMonths, RateType: domain.RateType(rec.RateType), DebitAccount: debit,
	})
	if err != nil {
		b.warn(g.line, "loan not created: %v", err)
		return LoanPlanRow{}, false, nil
	}
	if slices.ContainsFunc(loans, func(l domain.Loan) bool { return strings.EqualFold(l.Name, loan.Name) }) {
		b.warn(g.line, "loan not created: a loan named %q already exists", loan.Name)
		return LoanPlanRow{}, false, nil
	}
	if row.Version.ValidFrom.Before(disbursed) {
		b.warn(g.line, "loan not created: valid_from is before disbursed_on")
		return LoanPlanRow{}, false, nil
	}
	if info.create {
		b.openLoanAccount(g.rec.Account, principal, disbursed)
	}
	row.New, row.Loan, row.DebitRef = true, loan, debitRef
	return row, true, nil
}

// openLoanAccount makes a to-be-created liability account start at the disbursement, owing the principal.
func (b *builder) openLoanAccount(ref string, principal domain.Money, disbursed domain.Date) {
	for i := range b.plan.NewAccounts {
		na := &b.plan.NewAccounts[i]
		if na.Ref != ref {
			continue
		}
		if opening, err := domain.NewMoney(-principal.Minor(), principal.Currency()); err == nil {
			na.Account.OpeningBalance, na.Account.OpenedOn = opening, disbursed
		}
	}
}

// debitAccount resolves the debit account of a new loan against the stream refs, then the
// names of the active accounts. The returned ID is a placeholder while the account is still
// to be created.
func (b *builder) debitAccount(ref string) (domain.AccountID, string, bool) {
	if info := b.accts[ref]; info != nil && info.resolved() {
		return info.id, ref, true
	}
	for _, a := range b.active {
		if strings.EqualFold(a.Name, strings.TrimSpace(ref)) {
			return a.ID, a.Name, true
		}
	}
	return 0, "", false
}

// loansChange summarises the included loan rows for the plan's Changes.
func (p *ImportPlan) loansChange() (app.Change, bool) {
	created, versions := 0, 0
	for _, row := range p.Loans {
		if !row.Include || row.Module != "" {
			continue
		}
		versions++
		if row.New {
			created++
		}
	}
	if versions == 0 {
		return app.Change{}, false
	}
	return app.Change{
		Op: app.OpCreate, Entity: "loan",
		Fields:  map[string]any{"count": versions, "new": created},
		Summary: fmt.Sprintf("create/update %d loans", versions),
	}, true
}
