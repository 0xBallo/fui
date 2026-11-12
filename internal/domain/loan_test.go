package domain

import (
	"errors"
	"math/big"
	"testing"

	"pgregory.net/rapid"
)

// --- helpers for loan tests ---

func annualRate(t tb, bps int64) AnnualRate {
	t.Helper()
	r, err := NewAnnualRate(bps)
	if err != nil {
		t.Fatalf("NewAnnualRate(%d): %v", bps, err)
	}
	return r
}

func refLoan(t tb) Loan {
	t.Helper()
	eur := cur(t, "EUR")
	principal, _ := NewMoney(10_000_000, eur) // 100 000.00 EUR
	l, err := NewLoan(Loan{
		Kind:         LoanKindMortgage,
		Principal:    principal,
		DisbursedOn:  date(t, 2026, 1, 1),
		TermMonths:   12,
		RateType:     RateTypeFixed,
		DebitAccount: AccountID(1),
	})
	if err != nil {
		t.Fatalf("NewLoan: %v", err)
	}
	return l
}

// --- validation ---

// TestNewLoan_validation checks that NewLoan rejects invalid inputs.
func TestNewLoan_validation(t *testing.T) {
	t.Parallel()

	eur := cur(t, "EUR")
	goodPrincipal := money(t, 10_000_000, "EUR")
	goodDate := date(t, 2026, 1, 1)

	cases := []struct {
		name    string
		loan    Loan
		wantErr error
	}{
		{
			name: "valid",
			loan: Loan{
				Kind: LoanKindMortgage, Principal: goodPrincipal,
				DisbursedOn: goodDate, TermMonths: 12,
				RateType: RateTypeFixed, DebitAccount: 1,
			},
		},
		{
			name:    "zero kind",
			loan:    Loan{Principal: goodPrincipal, DisbursedOn: goodDate, TermMonths: 12, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrMissingField,
		},
		{
			name:    "invalid kind",
			loan:    Loan{Kind: "bank", Principal: goodPrincipal, DisbursedOn: goodDate, TermMonths: 12, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "zero principal currency",
			loan:    Loan{Kind: LoanKindLoan, DisbursedOn: goodDate, TermMonths: 12, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrMissingField,
		},
		{
			name:    "negative principal",
			loan:    Loan{Kind: LoanKindLoan, Principal: money(t, -1, "EUR"), DisbursedOn: goodDate, TermMonths: 12, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "zero principal",
			loan:    Loan{Kind: LoanKindLoan, Principal: money(t, 0, "EUR"), DisbursedOn: goodDate, TermMonths: 12, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "zero disbursed_on",
			loan:    Loan{Kind: LoanKindLoan, Principal: goodPrincipal, TermMonths: 12, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrMissingField,
		},
		{
			name:    "zero term",
			loan:    Loan{Kind: LoanKindLoan, Principal: goodPrincipal, DisbursedOn: goodDate, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "negative term",
			loan:    Loan{Kind: LoanKindLoan, Principal: goodPrincipal, DisbursedOn: goodDate, TermMonths: -1, RateType: RateTypeFixed, DebitAccount: 1},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "invalid rate type",
			loan:    Loan{Kind: LoanKindLoan, Principal: goodPrincipal, DisbursedOn: goodDate, TermMonths: 12, RateType: "unknown", DebitAccount: 1},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "zero debit account",
			loan:    Loan{Kind: LoanKindLoan, Principal: goodPrincipal, DisbursedOn: goodDate, TermMonths: 12, RateType: RateTypeFixed},
			wantErr: ErrMissingField,
		},
	}

	_ = eur // used implicitly through the money helper
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewLoan(tc.loan)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got err=%v, want errors.Is(%v)", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- AnnualRate ---

// TestAnnualRate_validation checks positive-only constraint.
func TestAnnualRate_validation(t *testing.T) {
	t.Parallel()
	if _, err := NewAnnualRate(0); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("zero bps: got %v, want ErrInvalidValue", err)
	}
	if _, err := NewAnnualRate(-1); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("negative bps: got %v, want ErrInvalidValue", err)
	}
	if r, err := NewAnnualRate(350); err != nil || r.BasisPoints() != 350 {
		t.Errorf("valid bps: got %v, %v", r, err)
	}
}

// TestAnnualRate_string verifies the display format.
func TestAnnualRate_string(t *testing.T) {
	t.Parallel()
	cases := []struct {
		bps  int64
		want string
	}{
		{350, "3.50%"},
		{100, "1.00%"},
		{1250, "12.50%"},
		{1, "0.01%"},
		{1000, "10.00%"},
	}
	for _, tc := range cases {
		r, _ := NewAnnualRate(tc.bps)
		if got := r.String(); got != tc.want {
			t.Errorf("AnnualRate(%d).String() = %q, want %q", tc.bps, got, tc.want)
		}
	}
}

// --- ComputeSchedule table tests ---

// TestComputeSchedule_knownValues verifies a 3-month schedule against
// independently computed reference values.
//
// Source of expected values: Python fractions module with half-away-from-zero
// rounding (identical to domain.RoundDiv).
//
//	Capital: 3 000.00 EUR (300 000 cents)
//	Rate:    5.00% nominal annual (500 basis points)
//	Term:    3 months
//	Monthly payment: 100 834 cents (1 008.34 EUR)
//
// Month | Interest | Principal | Remaining
// ------+----------+-----------+----------
//
//	1 |     1250 |     99584 |    200416
//	2 |      835 |     99999 |    100417
//	3 |      418 |    100417 |         0
func TestComputeSchedule_knownValues(t *testing.T) {
	t.Parallel()

	eur := cur(t, "EUR")
	principal, _ := NewMoney(300_000, eur) // 3 000.00 EUR
	loan, _ := NewLoan(Loan{
		Kind:         LoanKindLoan,
		Principal:    principal,
		DisbursedOn:  date(t, 2026, 1, 1),
		TermMonths:   3,
		RateType:     RateTypeFixed,
		DebitAccount: AccountID(1),
	})

	r := annualRate(t, 500)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 3, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	if len(sch.Installments) != 3 {
		t.Fatalf("expected 3 installments, got %d", len(sch.Installments))
	}

	type row struct {
		interest, principal, remaining int64
		dueY, dueM, dueD               int
	}
	want := []row{
		{1250, 99_584, 200_416, 2026, 2, 1},
		{835, 99_999, 100_417, 2026, 3, 1},
		{418, 100_417, 0, 2026, 4, 1},
	}

	for i, w := range want {
		inst := sch.Installments[i]
		if inst.Interest.Minor() != w.interest {
			t.Errorf("[%d] interest: got %d, want %d", i, inst.Interest.Minor(), w.interest)
		}
		if inst.Principal.Minor() != w.principal {
			t.Errorf("[%d] principal: got %d, want %d", i, inst.Principal.Minor(), w.principal)
		}
		if inst.Remaining.Minor() != w.remaining {
			t.Errorf("[%d] remaining: got %d, want %d", i, inst.Remaining.Minor(), w.remaining)
		}
		wantDue, _ := NewDate(w.dueY, w.dueM, w.dueD)
		if inst.Due != wantDue {
			t.Errorf("[%d] due: got %s, want %s", i, inst.Due, wantDue)
		}
	}
}

// TestComputeSchedule_largeKnownValues verifies the 100 000 EUR, 3.50%, 12
// months reference case computed with Python fractions (half-away-from-zero).
//
// Source: Python fractions module, same rounding as domain.RoundDiv.
//
// Month | Interest | Principal | Remaining
// ------+----------+-----------+-----------
//
//	 1 |    29167 |    820049 |   9179951
//	 2 |    26775 |    822441 |   8357510
//	 3 |    24376 |    824840 |   7532670
//	 4 |    21970 |    827246 |   6705424
//	 5 |    19557 |    829659 |   5875765
//	 6 |    17138 |    832078 |   5043687
//	 7 |    14711 |    834505 |   4209182
//	 8 |    12277 |    836939 |   3372243
//	 9 |     9836 |    839380 |   2532863
//	10 |     7388 |    841828 |   1691035
//	11 |     4932 |    844284 |    846751
//	12 |     2470 |    846751 |         0
func TestComputeSchedule_largeKnownValues(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	type row struct{ interest, principal, remaining int64 }
	want := []row{
		{29167, 820049, 9179951},
		{26775, 822441, 8357510},
		{24376, 824840, 7532670},
		{21970, 827246, 6705424},
		{19557, 829659, 5875765},
		{17138, 832078, 5043687},
		{14711, 834505, 4209182},
		{12277, 836939, 3372243},
		{9836, 839380, 2532863},
		{7388, 841828, 1691035},
		{4932, 844284, 846751},
		{2470, 846751, 0},
	}

	for i, w := range want {
		inst := sch.Installments[i]
		if inst.Interest.Minor() != w.interest {
			t.Errorf("[%d] interest: got %d, want %d", i, inst.Interest.Minor(), w.interest)
		}
		if inst.Principal.Minor() != w.principal {
			t.Errorf("[%d] principal: got %d, want %d", i, inst.Principal.Minor(), w.principal)
		}
		if inst.Remaining.Minor() != w.remaining {
			t.Errorf("[%d] remaining: got %d, want %d", i, inst.Remaining.Minor(), w.remaining)
		}
	}
}

// TestComputeSchedule_bankInstallmentOverride verifies that a bank-stated
// payment amount replaces the computed constant.
//
//	Capital: 3 000.00 EUR, rate 5%, term 3 months, bank states 102000 cents.
//	Month 1: interest = round(300000 * 500/120000) = 1250, principal = 100750, remaining = 199250
//	Month 2: interest = round(199250 * 500/120000) = 830, principal = 101170, remaining = 98080
//	Month 3 (last): principal = 98080, remaining = 0
func TestComputeSchedule_bankInstallmentOverride(t *testing.T) {
	t.Parallel()

	eur := cur(t, "EUR")
	principal, _ := NewMoney(300_000, eur)
	loan, _ := NewLoan(Loan{
		Kind: LoanKindLoan, Principal: principal,
		DisbursedOn: date(t, 2026, 1, 1), TermMonths: 3,
		RateType: RateTypeFixed, DebitAccount: 1,
	})

	r := annualRate(t, 500)
	bankPayment := int64(102_000)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 3, bankPayment)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	// Months 0 and 1: total payment = bankPayment.
	for _, i := range []int{0, 1} {
		inst := sch.Installments[i]
		total := inst.Principal.Minor() + inst.Interest.Minor()
		if total != bankPayment {
			t.Errorf("month[%d] total: got %d, want %d", i, total, bankPayment)
		}
	}

	// Last remaining must be 0.
	last := sch.Installments[2]
	if last.Remaining.Minor() != 0 {
		t.Errorf("last remaining: got %d, want 0", last.Remaining.Minor())
	}

	// Sum of principals must equal capital.
	var sumP int64
	for _, inst := range sch.Installments {
		sumP += inst.Principal.Minor()
	}
	if sumP != 300_000 {
		t.Errorf("principal sum: got %d, want 300000", sumP)
	}
}

// TestComputeSchedule_singleInstallment verifies a 1-month schedule.
func TestComputeSchedule_singleInstallment(t *testing.T) {
	t.Parallel()

	eur := cur(t, "EUR")
	principal, _ := NewMoney(100_000, eur) // 1 000.00 EUR
	loan, _ := NewLoan(Loan{
		Kind: LoanKindLoan, Principal: principal,
		DisbursedOn: date(t, 2026, 1, 1), TermMonths: 1,
		RateType: RateTypeFixed, DebitAccount: 1,
	})

	r := annualRate(t, 600) // 6%
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 1, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}
	if len(sch.Installments) != 1 {
		t.Fatalf("expected 1 installment, got %d", len(sch.Installments))
	}
	inst := sch.Installments[0]
	// Interest = round(100000 * 600/120000) = round(500) = 500
	if inst.Interest.Minor() != 500 {
		t.Errorf("interest: got %d, want 500", inst.Interest.Minor())
	}
	if inst.Principal.Minor() != 100_000 {
		t.Errorf("principal: got %d, want 100000", inst.Principal.Minor())
	}
	if inst.Remaining.Minor() != 0 {
		t.Errorf("remaining: got %d, want 0", inst.Remaining.Minor())
	}
}

// --- SplitInstallment ---

// TestSplitInstallment verifies that SplitInstallment returns the three fields correctly.
func TestSplitInstallment(t *testing.T) {
	t.Parallel()

	eur := cur(t, "EUR")
	mkM := func(c int64) Money { m, _ := NewMoney(c, eur); return m }

	inst := Installment{
		Due:       date(t, 2026, 2, 1),
		Principal: mkM(820049),
		Interest:  mkM(29167),
		Fees:      mkM(500),
		Remaining: mkM(9179951),
	}

	p, i, f := SplitInstallment(inst)
	if p != inst.Principal {
		t.Errorf("principal mismatch: got %v, want %v", p, inst.Principal)
	}
	if i != inst.Interest {
		t.Errorf("interest mismatch: got %v, want %v", i, inst.Interest)
	}
	if f != inst.Fees {
		t.Errorf("fees mismatch: got %v, want %v", f, inst.Fees)
	}
}

// TestInstallmentTotal verifies the Total() helper.
func TestInstallmentTotal(t *testing.T) {
	t.Parallel()
	eur := cur(t, "EUR")
	inst := Installment{
		Due:       date(t, 2026, 2, 1),
		Principal: money(t, 820049, "EUR"),
		Interest:  money(t, 29167, "EUR"),
		Fees:      money(t, 500, "EUR"),
		Remaining: money(t, 9179951, "EUR"),
	}
	total, err := inst.Total()
	if err != nil {
		t.Fatalf("Total: %v", err)
	}
	wantMinor := int64(820049 + 29167 + 500)
	if total.Minor() != wantMinor {
		t.Errorf("Total.Minor() = %d, want %d", total.Minor(), wantMinor)
	}
	if total.Currency() != money(t, 0, "EUR").Currency() {
		t.Errorf("Total.Currency() = %v, want EUR", total.Currency())
	}
	_ = eur
}

// --- ApplyEvent ---

// TestApplyEvent_rateChange verifies that a rate-change event produces a new
// schedule from the event date covering the remaining installments.
func TestApplyEvent_rateChange(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	// Rate changes after month 3 (installments 1-3 are Feb, Mar, Apr 2026).
	// Event date: the due date of month 3 (2026-04-01) means months 4-12 get new rate.
	newR := annualRate(t, 500)
	eventDate := sch.Installments[2].Due // 2026-04-01
	event := LoanEvent{
		LoanID:    loan.ID,
		Kind:      EventKindRateChange,
		EventDate: eventDate,
		NewRate:   &newR,
	}

	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}

	if newSch.Rate != newR {
		t.Errorf("new rate: got %v, want %v", newSch.Rate, newR)
	}

	// Should cover the remaining 9 months (12 - 3).
	if len(newSch.Installments) != 9 {
		t.Errorf("installment count: got %d, want 9", len(newSch.Installments))
	}

	// Capital for the new schedule is the remaining after month 3: 7 532 670.
	firstInst := newSch.Installments[0]
	gotCapital := firstInst.Principal.Minor() + firstInst.Remaining.Minor()
	wantCapital := int64(7_532_670)
	if gotCapital != wantCapital {
		t.Errorf("first principal+remaining: got %d, want %d", gotCapital, wantCapital)
	}

	// Final remaining of new schedule = 0.
	last := newSch.Installments[len(newSch.Installments)-1]
	if last.Remaining.Minor() != 0 {
		t.Errorf("last remaining: got %d, want 0", last.Remaining.Minor())
	}
}

// TestApplyEvent_prepayment_reduceInstallment verifies a prepayment that
// reduces the monthly payment while keeping the term.
func TestApplyEvent_prepayment_reduceInstallment(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	// Prepay 1 000 000 cents (10 000 EUR) right after month 6.
	prepaid := money(t, 1_000_000, "EUR")
	mode := PrepaymentReduceInstallment
	eventDate := sch.Installments[5].Due // 2026-07-01
	event := LoanEvent{
		LoanID:        loan.ID,
		Kind:          EventKindPrepayment,
		EventDate:     eventDate,
		PrepaidAmount: &prepaid,
		PrepayMode:    &mode,
	}

	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}

	// New schedule covers remaining 6 months.
	if len(newSch.Installments) != 6 {
		t.Errorf("installment count: got %d, want 6", len(newSch.Installments))
	}

	last := newSch.Installments[len(newSch.Installments)-1]
	if last.Remaining.Minor() != 0 {
		t.Errorf("last remaining: got %d, want 0", last.Remaining.Minor())
	}
}

// TestApplyEvent_installmentsBefore_unchanged verifies that the original
// schedule's installment slice is not mutated by ApplyEvent.
func TestApplyEvent_installmentsBefore_unchanged(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	snap := make([]Installment, 6)
	copy(snap, sch.Installments[:6])

	newR := annualRate(t, 500)
	event := LoanEvent{
		LoanID:    loan.ID,
		Kind:      EventKindRateChange,
		EventDate: sch.Installments[5].Due,
		NewRate:   &newR,
	}
	if _, err := ApplyEvent(event, sch, loan); err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}

	for i, want := range snap {
		got := sch.Installments[i]
		if got.Principal.Minor() != want.Principal.Minor() {
			t.Errorf("installment[%d].Principal mutated: got %d, want %d",
				i, got.Principal.Minor(), want.Principal.Minor())
		}
		if got.Interest.Minor() != want.Interest.Minor() {
			t.Errorf("installment[%d].Interest mutated: got %d, want %d",
				i, got.Interest.Minor(), want.Interest.Minor())
		}
	}
}

// TestApplyEvent_renegotiation verifies that a renegotiation can change both
// rate and term.
func TestApplyEvent_renegotiation(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	// Renegotiate after month 3: new rate 4%, new term 12 months (longer).
	newR := annualRate(t, 400)
	newTerm := 12
	event := LoanEvent{
		LoanID:        loan.ID,
		Kind:          EventKindRenegotiation,
		EventDate:     sch.Installments[2].Due,
		NewRate:       &newR,
		NewTermMonths: &newTerm,
	}
	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent renegotiation: %v", err)
	}

	if len(newSch.Installments) != 12 {
		t.Errorf("term: got %d, want 12", len(newSch.Installments))
	}
	last := newSch.Installments[len(newSch.Installments)-1]
	if last.Remaining.Minor() != 0 {
		t.Errorf("last remaining: got %d, want 0", last.Remaining.Minor())
	}
}

// TestApplyEvent_missingNewRate verifies error when rate change omits new rate.
func TestApplyEvent_missingNewRate(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)

	event := LoanEvent{
		LoanID:    loan.ID,
		Kind:      EventKindRateChange,
		EventDate: sch.Installments[2].Due,
		// NewRate omitted
	}
	_, err := ApplyEvent(event, sch, loan)
	if !errors.Is(err, ErrMissingField) {
		t.Errorf("expected ErrMissingField, got %v", err)
	}
}

// TestApplyEvent_eventAfterLastInstallment verifies error when event date is
// past all installments.
func TestApplyEvent_eventAfterLastInstallment(t *testing.T) {
	t.Parallel()

	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)

	newR := annualRate(t, 500)
	event := LoanEvent{
		LoanID:    loan.ID,
		Kind:      EventKindRateChange,
		EventDate: date(t, 2030, 1, 1), // way past the last installment
		NewRate:   &newR,
	}
	_, err := ApplyEvent(event, sch, loan)
	if !errors.Is(err, ErrInvalidValue) {
		t.Errorf("expected ErrInvalidValue for past-end event date, got %v", err)
	}
}

// --- Property-based tests ---

// TestProperty_principalSumEqualsCapital verifies that the sum of all
// principal components in a computed schedule equals the loan principal.
func TestProperty_principalSumEqualsCapital(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		capitalCents := rapid.Int64Range(100, 100_000_000).Draw(rt, "capital")
		bps := rapid.Int64Range(1, 2500).Draw(rt, "bps")
		months := rapid.IntRange(1, 360).Draw(rt, "months")

		eur := cur(rt, "EUR")
		principal, err := NewMoney(capitalCents, eur)
		if err != nil {
			rt.Skip()
		}
		r, err := NewAnnualRate(bps)
		if err != nil {
			rt.Skip()
		}
		loan, err := NewLoan(Loan{
			Kind: LoanKindLoan, Principal: principal,
			DisbursedOn: date(rt, 2020, 1, 1), TermMonths: months,
			RateType: RateTypeFixed, DebitAccount: 1,
		})
		if err != nil {
			rt.Skip()
		}

		sch, err := ComputeSchedule(loan, date(rt, 2020, 1, 1), r, months, 0)
		if err != nil {
			rt.Fatal("ComputeSchedule failed:", err)
		}

		var sumPrincipal int64
		for _, inst := range sch.Installments {
			sumPrincipal += inst.Principal.Minor()
		}
		if sumPrincipal != capitalCents {
			rt.Errorf("sum of principals %d != capital %d", sumPrincipal, capitalCents)
		}
	})
}

// TestProperty_finalResidualIsZero verifies that the last installment's
// Remaining field is always zero.
func TestProperty_finalResidualIsZero(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		capitalCents := rapid.Int64Range(100, 10_000_000).Draw(rt, "capital")
		bps := rapid.Int64Range(1, 2000).Draw(rt, "bps")
		months := rapid.IntRange(1, 240).Draw(rt, "months")

		eur := cur(rt, "EUR")
		principal, _ := NewMoney(capitalCents, eur)
		r, _ := NewAnnualRate(bps)
		loan, err := NewLoan(Loan{
			Kind: LoanKindLoan, Principal: principal,
			DisbursedOn: date(rt, 2020, 1, 1), TermMonths: months,
			RateType: RateTypeFixed, DebitAccount: 1,
		})
		if err != nil {
			rt.Skip()
		}

		sch, err := ComputeSchedule(loan, date(rt, 2020, 1, 1), r, months, 0)
		if err != nil {
			rt.Fatal("ComputeSchedule failed:", err)
		}

		last := sch.Installments[len(sch.Installments)-1]
		if last.Remaining.Minor() != 0 {
			rt.Errorf("final Remaining = %d, want 0", last.Remaining.Minor())
		}
	})
}

// TestProperty_eventPreservesResidualZero verifies that after a rate-change
// event the new schedule's last installment still has Remaining = 0.
func TestProperty_eventPreservesResidualZero(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		capitalCents := rapid.Int64Range(1000, 10_000_000).Draw(rt, "capital")
		bps := rapid.Int64Range(1, 2000).Draw(rt, "bps")
		months := rapid.IntRange(3, 120).Draw(rt, "months")
		eventIdx := rapid.IntRange(1, months-1).Draw(rt, "event_idx")

		eur := cur(rt, "EUR")
		principal, _ := NewMoney(capitalCents, eur)
		r, _ := NewAnnualRate(bps)
		loan, err := NewLoan(Loan{
			Kind: LoanKindLoan, Principal: principal,
			DisbursedOn: date(rt, 2020, 1, 1), TermMonths: months,
			RateType: RateTypeVariable, DebitAccount: 1,
		})
		if err != nil {
			rt.Skip()
		}

		sch, err := ComputeSchedule(loan, date(rt, 2020, 1, 1), r, months, 0)
		if err != nil {
			rt.Fatal("ComputeSchedule failed:", err)
		}

		eventDate := sch.Installments[eventIdx-1].Due
		newBps := rapid.Int64Range(1, 2000).Draw(rt, "new_bps")
		newR, _ := NewAnnualRate(newBps)
		event := LoanEvent{
			LoanID:    loan.ID,
			Kind:      EventKindRateChange,
			EventDate: eventDate,
			NewRate:   &newR,
		}

		newSch, err := ApplyEvent(event, sch, loan)
		if err != nil {
			rt.Fatal("ApplyEvent failed:", err)
		}

		if len(newSch.Installments) == 0 {
			rt.Fatal("empty new schedule")
		}
		last := newSch.Installments[len(newSch.Installments)-1]
		if last.Remaining.Minor() != 0 {
			rt.Errorf("final Remaining = %d, want 0 (capital=%d bps=%d months=%d eventIdx=%d)",
				last.Remaining.Minor(), capitalCents, bps, months, eventIdx)
		}
	})
}

// TestProperty_installmentsBeforeEventUnchanged is a property-based version
// of TestApplyEvent_installmentsBefore_unchanged.
func TestProperty_installmentsBeforeEventUnchanged(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		capitalCents := rapid.Int64Range(1000, 5_000_000).Draw(rt, "capital")
		bps := rapid.Int64Range(1, 1000).Draw(rt, "bps")
		months := rapid.IntRange(3, 60).Draw(rt, "months")
		eventIdx := rapid.IntRange(1, months-1).Draw(rt, "event_idx")

		eur := cur(rt, "EUR")
		principal, _ := NewMoney(capitalCents, eur)
		r, _ := NewAnnualRate(bps)
		loan, err := NewLoan(Loan{
			Kind: LoanKindLoan, Principal: principal,
			DisbursedOn: date(rt, 2020, 1, 1), TermMonths: months,
			RateType: RateTypeFixed, DebitAccount: 1,
		})
		if err != nil {
			rt.Skip()
		}

		sch, err := ComputeSchedule(loan, date(rt, 2020, 1, 1), r, months, 0)
		if err != nil {
			rt.Fatal("ComputeSchedule failed:", err)
		}

		snap := make([]Installment, eventIdx)
		copy(snap, sch.Installments[:eventIdx])

		eventDate := sch.Installments[eventIdx-1].Due
		newBps := rapid.Int64Range(1, 1000).Draw(rt, "new_bps")
		newR, _ := NewAnnualRate(newBps)
		event := LoanEvent{
			LoanID:    loan.ID,
			Kind:      EventKindRateChange,
			EventDate: eventDate,
			NewRate:   &newR,
		}

		if _, err := ApplyEvent(event, sch, loan); err != nil {
			rt.Fatal("ApplyEvent failed:", err)
		}

		for i, want := range snap {
			got := sch.Installments[i]
			if got.Principal.Minor() != want.Principal.Minor() {
				rt.Errorf("installment[%d].Principal mutated: got %d, want %d",
					i, got.Principal.Minor(), want.Principal.Minor())
			}
			if got.Interest.Minor() != want.Interest.Minor() {
				rt.Errorf("installment[%d].Interest mutated: got %d, want %d",
					i, got.Interest.Minor(), want.Interest.Minor())
			}
		}
	})
}

// --- ParseLoanKind / ParseRateType ---

// TestParseLoanKind verifies case-insensitive parsing and rejection of unknown kinds.
func TestParseLoanKind(t *testing.T) {
	t.Parallel()
	if k, err := ParseLoanKind("MORTGAGE"); err != nil || k != LoanKindMortgage {
		t.Errorf("ParseLoanKind(MORTGAGE) = %v, %v", k, err)
	}
	if _, err := ParseLoanKind("bank"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ParseLoanKind(bank): got %v, want ErrInvalidValue", err)
	}
	if _, err := ParseLoanKind(""); !errors.Is(err, ErrMissingField) {
		t.Errorf("ParseLoanKind(\"\"): got %v, want ErrMissingField", err)
	}
}

// TestParseRateType verifies case-insensitive parsing and rejection of unknown types.
func TestParseRateType(t *testing.T) {
	t.Parallel()
	if rt, err := ParseRateType("Variable"); err != nil || rt != RateTypeVariable {
		t.Errorf("ParseRateType(Variable) = %v, %v", rt, err)
	}
	if _, err := ParseRateType("unknown"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ParseRateType(unknown): got %v, want ErrInvalidValue", err)
	}
}

// --- enum Valid() coverage ---

// TestScheduleOriginValid checks the known values and rejects garbage.
func TestScheduleOriginValid(t *testing.T) {
	t.Parallel()
	if !ScheduleOriginImported.Valid() || !ScheduleOriginComputed.Valid() {
		t.Error("known schedule origins must be valid")
	}
	if ScheduleOrigin("bogus").Valid() {
		t.Error("unknown schedule origin must not be valid")
	}
}

// TestLoanEventKindValid checks the known values and rejects garbage.
func TestLoanEventKindValid(t *testing.T) {
	t.Parallel()
	for _, k := range []LoanEventKind{EventKindRateChange, EventKindRenegotiation, EventKindPrepayment} {
		if !k.Valid() {
			t.Errorf("%v must be valid", k)
		}
	}
	if LoanEventKind("bogus").Valid() {
		t.Error("unknown event kind must not be valid")
	}
}

// TestPrepaymentModeValid checks the known values and rejects garbage.
func TestPrepaymentModeValid(t *testing.T) {
	t.Parallel()
	for _, m := range []PrepaymentMode{PrepaymentReduceInstallment, PrepaymentReduceTerm} {
		if !m.Valid() {
			t.Errorf("%v must be valid", m)
		}
	}
	if PrepaymentMode("bogus").Valid() {
		t.Error("unknown prepayment mode must not be valid")
	}
}

// --- frenchInstallmentAmount edge cases ---

// TestFrenchInstallmentAmount_zeroRate verifies the zero-interest degenerate case R = C/n.
func TestFrenchInstallmentAmount_zeroRate(t *testing.T) {
	t.Parallel()
	zero := new(big.Rat)
	r, err := frenchInstallmentAmount(1200, zero, 12)
	if err != nil {
		t.Fatalf("frenchInstallmentAmount: %v", err)
	}
	if r != 100 {
		t.Errorf("got %d, want 100", r)
	}
}

// TestFrenchInstallmentAmount_invalidInputs verifies the guard clauses.
func TestFrenchInstallmentAmount_invalidInputs(t *testing.T) {
	t.Parallel()
	rate := monthlyRate(350)
	if _, err := frenchInstallmentAmount(1000, rate, 0); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("n=0: got %v, want ErrInvalidValue", err)
	}
	if _, err := frenchInstallmentAmount(-1, rate, 12); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("negative capital: got %v, want ErrInvalidValue", err)
	}
}

// TestInstallmentTotal_currencyMismatch verifies Total() propagates a
// currency-mismatch error from the underlying Money arithmetic.
func TestInstallmentTotal_currencyMismatch(t *testing.T) {
	t.Parallel()
	inst := Installment{
		Due:       date(t, 2026, 2, 1),
		Principal: money(t, 100, "EUR"),
		Interest:  money(t, 10, "USD"),
		Fees:      money(t, 0, "EUR"),
	}
	if _, err := inst.Total(); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("got %v, want ErrCurrencyMismatch", err)
	}
}

// --- ApplyEvent error paths ---

// TestApplyEvent_invalidKind verifies the event-kind enum guard.
func TestApplyEvent_invalidKind(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	event := LoanEvent{LoanID: loan.ID, Kind: "bogus", EventDate: sch.Installments[2].Due}
	if _, err := ApplyEvent(event, sch, loan); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("got %v, want ErrInvalidValue", err)
	}
}

// TestApplyEvent_missingEventDate verifies the event-date guard.
func TestApplyEvent_missingEventDate(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	newR := annualRate(t, 500)
	event := LoanEvent{LoanID: loan.ID, Kind: EventKindRateChange, NewRate: &newR}
	if _, err := ApplyEvent(event, sch, loan); !errors.Is(err, ErrMissingField) {
		t.Errorf("got %v, want ErrMissingField", err)
	}
}

// TestApplyEvent_eventBeforeFirstInstallment verifies the full-capital/full-term
// path (firstFuture == 0) when the event predates every installment.
func TestApplyEvent_eventBeforeFirstInstallment(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)

	newR := annualRate(t, 500)
	event := LoanEvent{
		LoanID: loan.ID, Kind: EventKindRateChange,
		EventDate: date(t, 2026, 1, 1), // before the first installment (2026-02-01)
		NewRate:   &newR,
	}
	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}
	if len(newSch.Installments) != 12 {
		t.Errorf("installment count: got %d, want 12", len(newSch.Installments))
	}
	var sumP int64
	for _, inst := range newSch.Installments {
		sumP += inst.Principal.Minor()
	}
	if sumP != loan.Principal.Minor() {
		t.Errorf("principal sum: got %d, want %d", sumP, loan.Principal.Minor())
	}
}

// TestApplyEvent_renegotiation_rateOnly verifies that omitting NewTermMonths
// keeps the remaining term unchanged.
func TestApplyEvent_renegotiation_rateOnly(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)

	newR := annualRate(t, 400)
	event := LoanEvent{
		LoanID: loan.ID, Kind: EventKindRenegotiation,
		EventDate: sch.Installments[2].Due, NewRate: &newR,
	}
	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}
	if len(newSch.Installments) != 9 {
		t.Errorf("installment count: got %d, want 9 (term unchanged)", len(newSch.Installments))
	}
	if newSch.Rate != newR {
		t.Errorf("rate: got %v, want %v", newSch.Rate, newR)
	}
}

// TestApplyEvent_prepayment_missingFields verifies the prepayment guards.
func TestApplyEvent_prepayment_missingFields(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	due := sch.Installments[5].Due
	prepaid := money(t, 1000, "EUR")
	mode := PrepaymentReduceInstallment
	badMode := PrepaymentMode("bogus")

	cases := []struct {
		name    string
		event   LoanEvent
		wantErr error
	}{
		{"missing amount", LoanEvent{Kind: EventKindPrepayment, EventDate: due, PrepayMode: &mode}, ErrMissingField},
		{"missing mode", LoanEvent{Kind: EventKindPrepayment, EventDate: due, PrepaidAmount: &prepaid}, ErrMissingField},
		{
			"invalid mode",
			LoanEvent{Kind: EventKindPrepayment, EventDate: due, PrepaidAmount: &prepaid, PrepayMode: &badMode},
			ErrInvalidValue,
		},
		{
			"non-positive amount",
			LoanEvent{Kind: EventKindPrepayment, EventDate: due, PrepaidAmount: ptr(money(t, 0, "EUR")), PrepayMode: &mode},
			ErrInvalidValue,
		},
		{
			"currency mismatch",
			LoanEvent{Kind: EventKindPrepayment, EventDate: due, PrepaidAmount: ptr(money(t, 1000, "USD")), PrepayMode: &mode},
			ErrCurrencyMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.event.LoanID = loan.ID
			if _, err := ApplyEvent(tc.event, sch, loan); !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(%v)", err, tc.wantErr)
			}
		})
	}
}

// TestApplyEvent_prepayment_fullPayoff verifies that prepaying the full
// residual capital settles the loan with an empty schedule.
func TestApplyEvent_prepayment_fullPayoff(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)

	eventDate := sch.Installments[5].Due
	residual := sch.Installments[5].Remaining
	mode := PrepaymentReduceTerm
	event := LoanEvent{
		LoanID: loan.ID, Kind: EventKindPrepayment,
		EventDate: eventDate, PrepaidAmount: &residual, PrepayMode: &mode,
	}
	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}
	if len(newSch.Installments) != 0 {
		t.Errorf("installment count: got %d, want 0 (full payoff)", len(newSch.Installments))
	}
}

// TestApplyEvent_prepayment_reduceTerm verifies that reducing the term keeps
// the payment at or below the original and never exceeds the prior term.
func TestApplyEvent_prepayment_reduceTerm(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}

	originalPayment := sch.Installments[0].Principal.Minor() + sch.Installments[0].Interest.Minor()

	prepaid := money(t, 1_000_000, "EUR")
	mode := PrepaymentReduceTerm
	eventDate := sch.Installments[5].Due // 2026-07-01
	event := LoanEvent{
		LoanID: loan.ID, Kind: EventKindPrepayment,
		EventDate: eventDate, PrepaidAmount: &prepaid, PrepayMode: &mode,
	}

	newSch, err := ApplyEvent(event, sch, loan)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}

	if len(newSch.Installments) == 0 || len(newSch.Installments) > 6 {
		t.Fatalf("installment count: got %d, want 1..6", len(newSch.Installments))
	}
	last := newSch.Installments[len(newSch.Installments)-1]
	if last.Remaining.Minor() != 0 {
		t.Errorf("last remaining: got %d, want 0", last.Remaining.Minor())
	}
	for i, inst := range newSch.Installments[:len(newSch.Installments)-1] {
		total := inst.Principal.Minor() + inst.Interest.Minor()
		if total > originalPayment {
			t.Errorf("installment[%d] payment %d exceeds original %d", i, total, originalPayment)
		}
	}
}

// ptr returns a pointer to v, for inline construction of LoanEvent optional fields.
func ptr[T any](v T) *T { return &v }

// --- Property: prepayment events ---

// TestProperty_prepaymentPreservesInvariants verifies, for random prepayment
// events in both modes, that the new schedule's principal sum equals the
// post-prepayment residual and the final installment's Remaining is zero.
func TestProperty_prepaymentPreservesInvariants(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		capitalCents := rapid.Int64Range(100_000, 10_000_000).Draw(rt, "capital")
		bps := rapid.Int64Range(1, 2000).Draw(rt, "bps")
		months := rapid.IntRange(3, 60).Draw(rt, "months")
		eventIdx := rapid.IntRange(1, months-1).Draw(rt, "event_idx")
		modePick := rapid.Bool().Draw(rt, "reduce_term")

		eur := cur(rt, "EUR")
		principal, _ := NewMoney(capitalCents, eur)
		r, _ := NewAnnualRate(bps)
		loan, err := NewLoan(Loan{
			Kind: LoanKindLoan, Principal: principal,
			DisbursedOn: date(rt, 2020, 1, 1), TermMonths: months,
			RateType: RateTypeFixed, DebitAccount: 1,
		})
		if err != nil {
			rt.Skip()
		}

		sch, err := ComputeSchedule(loan, date(rt, 2020, 1, 1), r, months, 0)
		if err != nil {
			rt.Fatal("ComputeSchedule failed:", err)
		}

		eventDate := sch.Installments[eventIdx-1].Due
		residual := sch.Installments[eventIdx-1].Remaining.Minor()
		if residual <= 0 {
			rt.Skip()
		}
		prepaidCents := rapid.Int64Range(1, residual).Draw(rt, "prepaid")
		prepaid, _ := NewMoney(prepaidCents, eur)
		mode := PrepaymentReduceInstallment
		if modePick {
			mode = PrepaymentReduceTerm
		}
		event := LoanEvent{
			LoanID: loan.ID, Kind: EventKindPrepayment,
			EventDate: eventDate, PrepaidAmount: &prepaid, PrepayMode: &mode,
		}

		newSch, err := ApplyEvent(event, sch, loan)
		if err != nil {
			rt.Fatal("ApplyEvent failed:", err)
		}

		wantResidual := residual - prepaidCents
		if wantResidual == 0 {
			if len(newSch.Installments) != 0 {
				rt.Errorf("expected empty schedule on full payoff, got %d installments", len(newSch.Installments))
			}
			return
		}

		if len(newSch.Installments) == 0 {
			rt.Fatal("empty new schedule")
		}
		var sumP int64
		for _, inst := range newSch.Installments {
			sumP += inst.Principal.Minor()
		}
		if sumP != wantResidual {
			rt.Errorf("sum of principals %d != residual %d", sumP, wantResidual)
		}
		last := newSch.Installments[len(newSch.Installments)-1]
		if last.Remaining.Minor() != 0 {
			rt.Errorf("final Remaining = %d, want 0", last.Remaining.Minor())
		}
		if mode == PrepaymentReduceInstallment && len(newSch.Installments) != months-eventIdx {
			rt.Errorf("reduce_installment must keep the term: got %d, want %d",
				len(newSch.Installments), months-eventIdx)
		}
		if mode == PrepaymentReduceTerm && len(newSch.Installments) > months-eventIdx {
			rt.Errorf("reduce_term must not lengthen the term: got %d, want <= %d",
				len(newSch.Installments), months-eventIdx)
		}
	})
}

// TestComputeSchedule_validation verifies the from_date and term_months guards.
func TestComputeSchedule_validation(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)

	if _, err := ComputeSchedule(loan, Date{}, r, 12, 0); !errors.Is(err, ErrMissingField) {
		t.Errorf("zero from_date: got %v, want ErrMissingField", err)
	}
	if _, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 0, 0); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("zero term_months: got %v, want ErrInvalidValue", err)
	}
}

// TestComputeSchedule_bankInstallmentBelowInterest verifies that a
// bank-stated payment too small to cover interest clamps the principal
// component to zero instead of going negative.
func TestComputeSchedule_bankInstallmentBelowInterest(t *testing.T) {
	t.Parallel()
	eur := cur(t, "EUR")
	principal, _ := NewMoney(300_000, eur)
	loan, _ := NewLoan(Loan{
		Kind: LoanKindLoan, Principal: principal,
		DisbursedOn: date(t, 2026, 1, 1), TermMonths: 3,
		RateType: RateTypeFixed, DebitAccount: 1,
	})
	r := annualRate(t, 500)
	sch, err := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 3, 1)
	if err != nil {
		t.Fatalf("ComputeSchedule: %v", err)
	}
	if sch.Installments[0].Principal.Minor() != 0 {
		t.Errorf("principal: got %d, want 0", sch.Installments[0].Principal.Minor())
	}
}

// TestApplyEvent_renegotiation_invalidNewTerm verifies the new_term_months guard.
func TestApplyEvent_renegotiation_invalidNewTerm(t *testing.T) {
	t.Parallel()
	loan := refLoan(t)
	r := annualRate(t, 350)
	sch, _ := ComputeSchedule(loan, date(t, 2026, 1, 1), r, 12, 0)

	badTerm := 0
	event := LoanEvent{
		LoanID: loan.ID, Kind: EventKindRenegotiation,
		EventDate: sch.Installments[2].Due, NewTermMonths: &badTerm,
	}
	if _, err := ApplyEvent(event, sch, loan); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("got %v, want ErrInvalidValue", err)
	}
}
