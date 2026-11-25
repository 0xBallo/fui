package importing

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// exportLoan is a loan with its stored schedule versions, oldest first.
type exportLoan struct {
	loan     domain.Loan
	versions []domain.Schedule
}

// loadLoans loads the loans selected by the export filter (all when it names no account)
// together with their schedule versions, ordered by loan ID. Loans without a stored
// schedule are skipped: a schedule record is what carries the loan metadata in the stream.
func (d *exportData) loadLoans(ctx context.Context, r app.Repos, f domain.TxFilter) error {
	loans, err := r.Loans().List(ctx)
	if err != nil {
		return fmt.Errorf("list loans: %w", err)
	}
	wanted := f.AccountIDs()
	for _, l := range loans {
		if len(wanted) > 0 && !slices.Contains(wanted, l.LiabilityAccountID) {
			continue
		}
		versions, err := loanVersions(ctx, r, l)
		if err != nil {
			return err
		}
		if len(versions) > 0 {
			d.loans = append(d.loans, exportLoan{loan: l, versions: versions})
		}
	}
	return nil
}

// loanVersions returns the schedule versions of l by ascending ValidFrom. LoanRepo cannot list
// versions, so it probes the disbursement date and the loan event dates, where versions start.
func loanVersions(ctx context.Context, r app.Repos, l domain.Loan) ([]domain.Schedule, error) {
	events, err := r.Loans().ListEvents(ctx, l.ID)
	if err != nil {
		return nil, fmt.Errorf("list events of loan %q: %w", l.Name, err)
	}
	starts := []domain.Date{l.DisbursedOn}
	for _, e := range events {
		starts = append(starts, e.EventDate)
	}
	slices.SortFunc(starts, domain.Date.Cmp)
	var out []domain.Schedule
	for _, from := range starts {
		s, err := r.Loans().ScheduleAt(ctx, l.ID, from)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("schedule of loan %q at %s: %w", l.Name, from, err)
		}
		if len(out) == 0 || out[len(out)-1].ValidFrom != s.ValidFrom {
			out = append(out, s)
		}
	}
	return out, nil
}

// loanAccounts returns the liability and debit accounts of the exported loans.
func (d exportData) loanAccounts() []domain.Account {
	var out []domain.Account
	for _, el := range d.loans {
		out = append(out, d.accounts[el.loan.LiabilityAccountID], d.accounts[el.loan.DebitAccount])
	}
	return out
}

// loanRecords returns, per loan, a schedule record for each version followed by its installments.
func (d exportData) loanRecords() []fuiproto.Record {
	var recs []fuiproto.Record
	for _, el := range d.loans {
		l := el.loan
		ref := accountRef(d.accounts[l.LiabilityAccountID])
		for _, v := range el.versions {
			recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeSchedule, Schedule: &fuiproto.ScheduleRecord{
				Account: ref, ValidFrom: v.ValidFrom.String(), Rate: v.Rate.BasisPoints(), RateType: fuiproto.RateType(l.RateType),
				DebitAccount: accountRef(d.accounts[l.DebitAccount]), Kind: fuiproto.LoanKind(l.Kind),
				Principal: l.Principal.Minor(), DisbursedOn: l.DisbursedOn.String(), TermMonths: l.TermMonths,
			}})
			for _, inst := range v.Installments {
				recs = append(recs, fuiproto.Record{Type: fuiproto.RecordTypeInstallment, Installment: &fuiproto.InstallmentRecord{
					Account: ref, DueDate: inst.Due.String(), Principal: inst.Principal.Minor(),
					Interest: inst.Interest.Minor(), Fees: inst.Fees.Minor(), Remaining: inst.Remaining.Minor(),
				}})
			}
		}
	}
	return recs
}
