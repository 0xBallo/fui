package domain

import (
	"fmt"
	"math/big"
)

// LoanKind classifies a financing contract.
type LoanKind string

// Known loan kinds.
const (
	LoanKindMortgage LoanKind = "mortgage"
	LoanKindLoan     LoanKind = "loan"
)

// Valid reports whether k is a known LoanKind.
func (k LoanKind) Valid() bool {
	switch k {
	case LoanKindMortgage, LoanKindLoan:
		return true
	}
	return false
}

// ParseLoanKind parses a loan kind (case-insensitive).
func ParseLoanKind(s string) (LoanKind, error) {
	return parseEnum("loan kind", s, LoanKind.Valid)
}

// RateType classifies how the nominal interest rate may change over time.
type RateType string

// Known rate types.
const (
	RateTypeFixed    RateType = "fixed"
	RateTypeVariable RateType = "variable"
	RateTypeMixed    RateType = "mixed"
)

// Valid reports whether rt is a known RateType.
func (rt RateType) Valid() bool {
	switch rt {
	case RateTypeFixed, RateTypeVariable, RateTypeMixed:
		return true
	}
	return false
}

// ParseRateType parses a rate type (case-insensitive).
func ParseRateType(s string) (RateType, error) {
	return parseEnum("rate type", s, RateType.Valid)
}

// LoanID identifies a Loan.
type LoanID int64

// ScheduleOrigin indicates how a Schedule was produced.
type ScheduleOrigin string

// Known schedule origins.
const (
	ScheduleOriginImported ScheduleOrigin = "imported"
	ScheduleOriginComputed ScheduleOrigin = "computed"
)

// Valid reports whether o is a known ScheduleOrigin.
func (o ScheduleOrigin) Valid() bool {
	switch o {
	case ScheduleOriginImported, ScheduleOriginComputed:
		return true
	}
	return false
}

// AnnualRate is a nominal annual interest rate stored as basis points (1 bp =
// 0.01%). A rate of 3.50% is represented as 350. This avoids float arithmetic
// in the domain while supporting the precision banks typically use (two decimal
// places on the annual percentage).
type AnnualRate int64

// NewAnnualRate returns the rate represented by basisPoints, which must be > 0.
func NewAnnualRate(basisPoints int64) (AnnualRate, error) {
	if basisPoints <= 0 {
		return 0, newError(ErrInvalidValue,
			map[string]any{"field": "annual_rate", "value": basisPoints},
			"annual rate must be positive, got %d basis points", basisPoints)
	}
	return AnnualRate(basisPoints), nil
}

// BasisPoints returns the raw basis-point value.
func (r AnnualRate) BasisPoints() int64 { return int64(r) }

// String renders the rate as a human-readable percentage, e.g. "3.50%".
func (r AnnualRate) String() string {
	whole := int64(r) / 100
	frac := int64(r) % 100
	return fmt.Sprintf("%d.%02d%%", whole, frac)
}

// Loan is a mortgage or personal loan held as a liability account. The balance
// (remaining principal) is always negative in net-worth terms.
type Loan struct {
	ID           LoanID
	Kind         LoanKind
	Principal    Money // total disbursed amount, positive
	DisbursedOn  Date  // date the funds were released
	TermMonths   int   // original contractual duration in months
	RateType     RateType
	DebitAccount AccountID // account from which instalments are debited
}

// NewLoan validates l and returns it. Principal must be positive and in a
// non-zero currency; TermMonths must be ≥ 1.
func NewLoan(l Loan) (Loan, error) {
	if !l.Kind.Valid() {
		return Loan{}, enumError("kind", string(l.Kind))
	}
	if l.Principal.Currency().IsZero() {
		return Loan{}, missingField("principal")
	}
	if l.Principal.Minor() <= 0 {
		return Loan{}, newError(ErrInvalidValue,
			map[string]any{"field": "principal", "value": l.Principal.Minor()},
			"principal must be positive")
	}
	if l.DisbursedOn.IsZero() {
		return Loan{}, missingField("disbursed_on")
	}
	if l.TermMonths < 1 {
		return Loan{}, newError(ErrInvalidValue,
			map[string]any{"field": "term_months", "value": l.TermMonths},
			"term_months must be at least 1")
	}
	if !l.RateType.Valid() {
		return Loan{}, enumError("rate_type", string(l.RateType))
	}
	if l.DebitAccount == 0 {
		return Loan{}, missingField("debit_account")
	}
	return l, nil
}

// Installment is a single scheduled payment in an amortization plan.
//
// All Money fields share the currency of the loan's principal. Principal and
// Interest are always non-negative. Remaining is the outstanding capital after
// this installment is applied; it reaches zero on the last installment.
type Installment struct {
	Due       Date
	Principal Money
	Interest  Money
	Fees      Money
	Remaining Money
}

// Total returns the total payment: principal + interest + fees.
func (inst Installment) Total() (Money, error) {
	t, err := inst.Principal.Add(inst.Interest)
	if err != nil {
		return Money{}, err
	}
	return t.Add(inst.Fees)
}

// SplitInstallment returns the three money components of inst so the app layer
// can book them to separate transaction categories without touching the
// internal Money fields directly.
func SplitInstallment(inst Installment) (principal, interest, fees Money) {
	return inst.Principal, inst.Interest, inst.Fees
}

// Schedule is a versioned amortization plan for a Loan. ValidFrom is the date
// from which this version applies; installments before this date belong to
// earlier versions and are immutable. AnnualRate is the nominal annual rate
// (in basis points) in force for this version.
type Schedule struct {
	LoanID    LoanID
	ValidFrom Date
	Origin    ScheduleOrigin
	Rate      AnnualRate
	// Installments are in ascending Due order. The first installment's Due
	// date must be ≥ ValidFrom.
	Installments []Installment
}

// LoanEventKind classifies the fact recorded by a LoanEvent.
type LoanEventKind string

// Known event kinds.
const (
	EventKindRateChange    LoanEventKind = "rate_change"
	EventKindRenegotiation LoanEventKind = "renegotiation"
	EventKindPrepayment    LoanEventKind = "prepayment"
)

// Valid reports whether ek is a known LoanEventKind.
func (ek LoanEventKind) Valid() bool {
	switch ek {
	case EventKindRateChange, EventKindRenegotiation, EventKindPrepayment:
		return true
	}
	return false
}

// PrepaymentMode chooses whether a partial prepayment reduces the monthly
// installment or the remaining term.
type PrepaymentMode string

// Known prepayment modes.
const (
	PrepaymentReduceInstallment PrepaymentMode = "reduce_installment"
	PrepaymentReduceTerm        PrepaymentMode = "reduce_term"
)

// Valid reports whether m is a known PrepaymentMode.
func (m PrepaymentMode) Valid() bool {
	switch m {
	case PrepaymentReduceInstallment, PrepaymentReduceTerm:
		return true
	}
	return false
}

// LoanEvent is a fact that invalidates part of an existing schedule and
// requires a new version from EventDate onward.
type LoanEvent struct {
	LoanID    LoanID
	Kind      LoanEventKind
	EventDate Date

	// Fields used depending on Kind:
	NewRate       *AnnualRate     // rate_change, renegotiation
	NewTermMonths *int            // renegotiation (optional override)
	PrepaidAmount *Money          // prepayment
	PrepayMode    *PrepaymentMode // prepayment
}

// monthlyRate converts an annual rate in basis points to the periodic (monthly)
// rate as a *big.Rat. The formula is r_monthly = r_annual / 12 where
// r_annual = basisPoints / 10000. We keep it as a rational to avoid float
// arithmetic.
func monthlyRate(basisPoints int64) *big.Rat {
	// r = basisPoints / (10000 * 12)
	return new(big.Rat).SetFrac(
		big.NewInt(basisPoints),
		big.NewInt(10000*12),
	)
}

// frenchInstallmentAmount computes the constant payment R for a French
// amortization:
//
//	R = C · i / (1 − (1+i)^−n)
//
// where C is the remaining capital (minor units), i is the monthly rate and n
// is the number of remaining installments. The result is in minor units,
// rounded half-away-from-zero.
//
// When i == 0 the formula degenerates to R = C/n (zero-interest loan).
func frenchInstallmentAmount(capitalMinor int64, rate *big.Rat, n int) (int64, error) {
	if n <= 0 {
		return 0, newError(ErrInvalidValue, nil, "number of installments must be positive")
	}
	if capitalMinor < 0 {
		return 0, newError(ErrInvalidValue, nil, "capital must be non-negative")
	}
	C := new(big.Rat).SetInt64(capitalMinor)

	if rate.Sign() == 0 {
		// Zero-interest: R = C/n exactly.
		R := new(big.Rat).SetInt(C.Num()) // C is an integer so Denom == 1.
		R.Quo(R, new(big.Rat).SetInt64(int64(n)))
		return RoundDiv(R.Num(), R.Denom())
	}

	// (1+i)^n with big.Rat
	onePlusI := new(big.Rat).Add(new(big.Rat).SetInt64(1), rate)
	pow := ratPow(onePlusI, n) // (1+i)^n

	// (1+i)^-n = 1 / (1+i)^n
	invPow := new(big.Rat).Inv(pow) // 1/(1+i)^n

	// 1 - (1+i)^-n
	denom := new(big.Rat).Sub(new(big.Rat).SetInt64(1), invPow)

	// R = C * i / denom
	R := new(big.Rat).Mul(C, rate)
	R.Quo(R, denom)

	return RoundDiv(R.Num(), R.Denom())
}

// ratPow returns base^exp for exp ≥ 0.
func ratPow(base *big.Rat, exp int) *big.Rat {
	result := new(big.Rat).SetInt64(1)
	b := new(big.Rat).Set(base)
	for exp > 0 {
		if exp&1 == 1 {
			result.Mul(result, b)
		}
		b.Mul(b, b)
		exp >>= 1
	}
	return result
}
