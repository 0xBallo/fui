package importing_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// seedLoanV2 adds a rate-change event and a second schedule version at 2026-02-15 to seedLoan.
func (e env) seedLoanV2(t *testing.T) domain.Loan {
	t.Helper()
	loan := e.seedLoan(t)
	rate := domain.AnnualRate(250)
	err := e.store.WithTx(context.Background(), func(r app.Repos) error {
		if _, err := r.Loans().InsertEvent(context.Background(), domain.LoanEvent{
			LoanID: loan.ID, Kind: domain.EventKindRateChange, EventDate: date("2026-02-15"), NewRate: &rate,
		}); err != nil {
			return err
		}
		return r.Loans().AddScheduleVersion(context.Background(), domain.Schedule{
			LoanID: loan.ID, ValidFrom: date("2026-02-15"), Origin: domain.ScheduleOriginImported, Rate: rate,
			Installments: []domain.Installment{
				{Due: date("2026-03-01"), Principal: money(1000), Interest: money(3), Fees: money(1), Remaining: money(0)},
			},
		})
	})
	if err != nil {
		t.Fatalf("seed version: %v", err)
	}
	return loan
}

func exportLoanFUI(t *testing.T, e env, spec domain.TxFilterSpec) (string, []fuiproto.Record) {
	t.Helper()
	out, _ := export(t, e, importing.FormatFUI, spec)
	res, err := fuiproto.ValidateStream(fuiproto.DecodeJSONL(strings.NewReader(out)))
	if err != nil || len(res.RowErrors) != 0 {
		t.Fatalf("invalid stream: %v %+v\n%s", err, res.RowErrors, out)
	}
	return out, res.Records
}

func TestExportLoanVersions(t *testing.T) {
	e := newEnv(nil)
	e.seedLoanV2(t)
	e.seedTx(t, domain.Transaction{
		AccountID: e.accountID(t, "Current"), Date: date("2026-02-01"), Amount: money(-10), Description: "x",
		Status: domain.StatusBooked, Kind: domain.KindPayment,
	})
	out, recs := exportLoanFUI(t, e, domain.TxFilterSpec{})

	var types []fuiproto.RecordType
	var scheds []*fuiproto.ScheduleRecord
	var insts []*fuiproto.InstallmentRecord
	for _, r := range recs {
		types = append(types, r.Type)
		if r.Schedule != nil {
			scheds = append(scheds, r.Schedule)
		}
		if r.Installment != nil {
			insts = append(insts, r.Installment)
		}
	}
	wantTypes := []fuiproto.RecordType{
		fuiproto.RecordTypeMeta, fuiproto.RecordTypeAccount, fuiproto.RecordTypeAccount, fuiproto.RecordTypeSchedule,
		fuiproto.RecordTypeInstallment, fuiproto.RecordTypeInstallment, fuiproto.RecordTypeSchedule, fuiproto.RecordTypeInstallment,
		fuiproto.RecordTypeTransaction, fuiproto.RecordTypeBalance, fuiproto.RecordTypeEnd,
	}
	if diff := cmp.Diff(wantTypes, types); diff != "" {
		t.Fatalf("record order (-want +got):\n%s\n%s", diff, out)
	}
	meta := func(from string, rate int64) *fuiproto.ScheduleRecord {
		return &fuiproto.ScheduleRecord{
			Account: "Mortgage", ValidFrom: from, Rate: rate, RateType: fuiproto.RateTypeFixed, DebitAccount: "Current",
			Kind: fuiproto.LoanKindMortgage, Principal: 2000, DisbursedOn: "2026-01-01", TermMonths: 2,
		}
	}
	if diff := cmp.Diff([]*fuiproto.ScheduleRecord{meta("2026-01-01", 300), meta("2026-02-15", 250)}, scheds); diff != "" {
		t.Fatalf("schedules (-want +got):\n%s", diff)
	}
	wantInsts := []*fuiproto.InstallmentRecord{
		{Account: "Mortgage", DueDate: "2026-02-01", Principal: 1000, Interest: 5, Remaining: 1000},
		{Account: "Mortgage", DueDate: "2026-03-01", Principal: 1000, Interest: 2},
		{Account: "Mortgage", DueDate: "2026-03-01", Principal: 1000, Interest: 3, Fees: 1},
	}
	if diff := cmp.Diff(wantInsts, insts); diff != "" {
		t.Fatalf("installments (-want +got):\n%s", diff)
	}
	again, _ := exportLoanFUI(t, e, domain.TxFilterSpec{})
	if again != out {
		t.Fatal("export is not deterministic")
	}
}

func (e env) accountID(t *testing.T, name string) domain.AccountID {
	t.Helper()
	var id domain.AccountID
	err := e.store.Read(context.Background(), func(r app.Repos) error {
		accts, err := r.Accounts().List(context.Background())
		for _, a := range accts {
			if a.Name == name {
				id = a.ID
			}
		}
		return err
	})
	if err != nil || id == 0 {
		t.Fatalf("account %q: %v", name, err)
	}
	return id
}

func TestExportLoanAccountFilter(t *testing.T) {
	e := newEnv(nil)
	e.seedLoanV2(t)
	other := e.seedAccount(t, "Other", "", "")
	cases := []struct {
		name string
		acct domain.AccountID
		want bool
	}{
		{"liability selected", e.accountID(t, "Mortgage"), true},
		{"other account", other.ID, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := exportLoanFUI(t, e, domain.TxFilterSpec{AccountIDs: []domain.AccountID{tc.acct}})
			if got := strings.Contains(out, `"type":"schedule"`); got != tc.want {
				t.Fatalf("schedule exported = %v, want %v\n%s", got, tc.want, out)
			}
		})
	}
}

func TestExportWithoutLoansHasNoLoanRecords(t *testing.T) {
	out, _ := exportLoanFUI(t, seedExport(t), domain.TxFilterSpec{})
	if strings.Contains(out, `"schedule"`) || strings.Contains(out, `"installment"`) {
		t.Fatalf("unexpected loan records:\n%s", out)
	}
}

func TestExportedLoanPlansAsNew(t *testing.T) {
	src := newEnv(nil)
	src.seedLoanV2(t)
	_, recs := exportLoanFUI(t, src, domain.TxFilterSpec{})

	dst := newEnv(nil)
	p := dst.plan(t, stream("fui", "", recs[1:len(recs)-1]...), loansOn, createAccounts)
	if len(p.Loans) != 2 {
		t.Fatalf("loan rows = %d (warnings %q), want 2", len(p.Loans), warnings(p))
	}
	// The first row creates the loan; the later version attaches to that new loan.
	if !p.Loans[0].New || p.Loans[1].New || p.Loans[1].Loan.Name != "Mortgage" || !p.Loans[1].Include {
		t.Fatalf("rows = %+v", p.Loans)
	}
}
