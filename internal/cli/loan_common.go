package cli

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/lending"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// resolveDebitCurrency resolves ref among the active accounts and returns its currency, needed
// to parse an --import file's minor-unit amounts before the loan service even sees the account.
func resolveDebitCurrency(ctx context.Context, deps *Deps, ref string) (domain.Currency, error) {
	rows, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{})
	if err != nil {
		return domain.Currency{}, err
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Account.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Currency{}, fmt.Errorf("account: %w", err)
	}
	return rows[i].Account.Currency, nil
}

// parseInstallmentsFile reads a fui/1 JSONL file and returns its installment records, converted
// to domain.Installment in currency and sorted by due date. Every other record type (meta,
// schedule, account, end) is ignored: loan add only needs the amortization rows.
func parseInstallmentsFile(path string, currency domain.Currency) ([]domain.Installment, error) {
	fh, err := os.Open(path) // #nosec G304 -- path is a file the user gave on the command line.
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer fh.Close() //nolint:errcheck // read-only file

	var out []domain.Installment
	for rec, err := range fuiproto.DecodeJSONL(fh) {
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if rec.Installment == nil {
			continue
		}
		inst, err := installmentFromRecord(*rec.Installment, currency)
		if err != nil {
			return nil, fmt.Errorf("installment in %s: %w", path, err)
		}
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Due.Before(out[j].Due) })
	return out, nil
}

// accountNameByID resolves the debit account name for a loan: Loan.DebitAccount is only an ID,
// but render.LoanView wants the name the CLI already has handy from ListAccounts.
func accountNameByID(ctx context.Context, deps *Deps, id domain.AccountID) (string, error) {
	rows, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{IncludeArchived: true})
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Account.ID == id {
			return r.Account.Name, nil
		}
	}
	return "", nil
}

// buildLoanView turns a lending.Info (and its events, when loading a full lending.View) into the
// render.LoanView the loan command family renders.
func buildLoanView(ctx context.Context, deps *Deps, info lending.Info, events []domain.LoanEvent) (render.LoanView, error) {
	debitName, err := accountNameByID(ctx, deps, info.Loan.DebitAccount)
	if err != nil {
		return render.LoanView{}, err
	}
	return render.LoanView{Loan: info.Loan, Account: info.Account, DebitAccountName: debitName, Events: events}, nil
}

// installmentFromRecord converts one InstallmentRecord (minor-unit int64 amounts, per the fui/1
// protocol) into a domain.Installment in currency.
func installmentFromRecord(r fuiproto.InstallmentRecord, currency domain.Currency) (domain.Installment, error) {
	due, err := domain.ParseDate(r.DueDate)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("due_date: %w", err)
	}
	principal, err := domain.NewMoney(r.Principal, currency)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("principal: %w", err)
	}
	interest, err := domain.NewMoney(r.Interest, currency)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("interest: %w", err)
	}
	fees, err := domain.NewMoney(r.Fees, currency)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("fees: %w", err)
	}
	remaining, err := domain.NewMoney(r.Remaining, currency)
	if err != nil {
		return domain.Installment{}, fmt.Errorf("remaining: %w", err)
	}
	return domain.Installment{Due: due, Principal: principal, Interest: interest, Fees: fees, Remaining: remaining}, nil
}
