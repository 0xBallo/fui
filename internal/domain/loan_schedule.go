package domain

import (
	"math/big"
)

// ComputeSchedule builds a French amortization schedule for loan l starting
// from fromDate, using annualRate (basis points). The schedule always has
// exactly termMonths installments; due dates are computed by adding 1 month per
// step starting from fromDate+1 month (i.e. the first due date is fromDate +1
// month, clamped to month-end).
//
// The last installment absorbs any cent-rounding residual so that the sum of
// all principal components equals l.Principal exactly.
//
// If a bankInstallment is provided (> 0), it overrides the computed constant
// payment amount. This lets an imported schedule win over the computed one.
func ComputeSchedule(
	l Loan,
	fromDate Date,
	rate AnnualRate,
	termMonths int,
	bankInstallment int64, // 0 = compute; > 0 = use bank-stated payment amount
) (Schedule, error) {
	if fromDate.IsZero() {
		return Schedule{}, missingField("from_date")
	}
	if termMonths < 1 {
		return Schedule{}, newError(ErrInvalidValue,
			map[string]any{"field": "term_months", "value": termMonths},
			"term_months must be at least 1")
	}

	cur := l.Principal.Currency()
	mRate := monthlyRate(rate.BasisPoints())
	capitalMinor := l.Principal.Minor()

	R := bankInstallment
	if R <= 0 {
		var err error
		R, err = frenchInstallmentAmount(capitalMinor, mRate, termMonths)
		if err != nil {
			return Schedule{}, err
		}
	}

	installments, err := buildInstallments(fromDate, capitalMinor, mRate, termMonths, R, cur)
	if err != nil {
		return Schedule{}, err
	}

	return Schedule{
		LoanID:       l.ID,
		ValidFrom:    fromDate,
		Origin:       ScheduleOriginComputed,
		Rate:         rate,
		Installments: installments,
	}, nil
}

// buildInstallments computes the termMonths installments of a French
// amortization starting at fromDate+1 month, given the constant payment R.
// Each period's principal is R minus that period's interest, clamped to
// [0, remaining] so rounding of R never drives the residual capital negative;
// the last installment always repays exactly what is left.
func buildInstallments(fromDate Date, capitalMinor int64, mRate *big.Rat, termMonths int, R int64, cur Currency) ([]Installment, error) {
	remaining := capitalMinor
	installments := make([]Installment, termMonths)

	for i := range termMonths {
		dueDate, err := fromDate.AddMonths(i + 1)
		if err != nil {
			return nil, err
		}

		// Interest for this period: remaining * mRate, rounded half-away-from-zero.
		interestMinor, err := mulRatRound(remaining, mRate)
		if err != nil {
			return nil, err
		}

		var principalMinor int64
		switch {
		case i == termMonths-1:
			// Last installment: repay exactly what is left.
			principalMinor = remaining
		case R-interestMinor < 0:
			// Guard: payment amount is too small to cover interest (can
			// happen with extremely high rates or bank-stated amounts).
			principalMinor = 0
		case R-interestMinor > remaining:
			// Guard: rounding of R can overshoot the true remaining capital
			// a step early; clamp so remaining never goes negative. The
			// (possibly zero-payment) installments that follow still sum to
			// capital and the last one reports a final remaining of 0.
			principalMinor = remaining
		default:
			principalMinor = R - interestMinor
		}

		remaining -= principalMinor

		inst, err := buildInstallment(dueDate, principalMinor, interestMinor, 0, remaining, cur)
		if err != nil {
			return nil, err
		}
		installments[i] = inst
	}
	return installments, nil
}

// ApplyEvent returns a new Schedule version that takes effect from
// event.EventDate, leaving installments before that date intact (they remain in
// the previous schedule versions). The existing schedule's installments before
// the event date are NOT included in the returned Schedule; callers splice
// versions together.
//
// Supported events:
//   - EventKindRateChange: recompute from the residual capital at the event date.
//   - EventKindRenegotiation: recompute with optional new term.
//   - EventKindPrepayment: subtract prepaid amount then recompute (reduce
//     installment or reduce term, per event.PrepayMode).
func ApplyEvent(event LoanEvent, currentSchedule Schedule, loan Loan) (Schedule, error) {
	if !event.Kind.Valid() {
		return Schedule{}, enumError("event_kind", string(event.Kind))
	}
	if event.EventDate.IsZero() {
		return Schedule{}, missingField("event_date")
	}

	// Find the remaining capital at the event date: the Remaining field of
	// the last installment whose Due date is strictly before event.EventDate.
	capital, termLeft, firstFuture, err := residualAtDate(currentSchedule, event.EventDate)
	if err != nil {
		return Schedule{}, err
	}
	cur := loan.Principal.Currency()

	var outcome eventOutcome
	switch event.Kind {
	case EventKindRateChange:
		outcome, err = resolveRateChange(event, termLeft, capital)
	case EventKindRenegotiation:
		outcome, err = resolveRenegotiation(event, currentSchedule.Rate, termLeft, capital)
	case EventKindPrepayment:
		outcome, err = resolvePrepayment(event, currentSchedule, cur, termLeft, firstFuture, capital)
	}
	if err != nil {
		return Schedule{}, err
	}

	if outcome.settled {
		// The event pays off the debt in full: no installments remain.
		return Schedule{
			LoanID:    loan.ID,
			ValidFrom: event.EventDate,
			Origin:    ScheduleOriginComputed,
			Rate:      outcome.rate,
		}, nil
	}
	term := outcome.term
	if term < 1 {
		term = 1
	}

	// Build a synthetic Loan with the residual as principal.
	syntheticPrincipal, err := NewMoney(outcome.capital, cur)
	if err != nil {
		return Schedule{}, err
	}
	syntheticLoan := loan
	syntheticLoan.Principal = syntheticPrincipal

	sch, err := ComputeSchedule(syntheticLoan, event.EventDate, outcome.rate, term, 0)
	if err != nil {
		return Schedule{}, err
	}
	sch.LoanID = loan.ID
	return sch, nil
}

// eventOutcome is what a LoanEvent resolves to: the rate and capital a new
// schedule version should start from and, unless settled, the term it should
// cover. settled means the debt is fully paid off and no schedule is needed.
type eventOutcome struct {
	rate    AnnualRate
	term    int
	capital int64
	settled bool
}

// resolveRateChange applies an EventKindRateChange event: only the rate
// changes, the remaining term and capital are unaffected.
func resolveRateChange(event LoanEvent, termLeft int, capital int64) (eventOutcome, error) {
	if event.NewRate == nil {
		return eventOutcome{}, missingField("new_rate")
	}
	return eventOutcome{rate: *event.NewRate, term: termLeft, capital: capital}, nil
}

// resolveRenegotiation applies an EventKindRenegotiation event: the rate
// and/or term may both change; either left nil keeps the current value.
func resolveRenegotiation(event LoanEvent, currentRate AnnualRate, termLeft int, capital int64) (eventOutcome, error) {
	rate := currentRate
	if event.NewRate != nil {
		rate = *event.NewRate
	}
	term := termLeft
	if event.NewTermMonths != nil {
		if *event.NewTermMonths < 1 {
			return eventOutcome{}, newError(ErrInvalidValue,
				map[string]any{"field": "new_term_months", "value": *event.NewTermMonths},
				"new_term_months must be at least 1")
		}
		term = *event.NewTermMonths
	}
	return eventOutcome{rate: rate, term: term, capital: capital}, nil
}

// resolvePrepayment applies an EventKindPrepayment event: it subtracts the
// prepaid amount from capital and, per event.PrepayMode, either keeps the
// term (letting the payment shrink) or keeps the payment (shortening the
// term). A prepayment that reaches zero capital settles the debt.
func resolvePrepayment(
	event LoanEvent, sch Schedule, cur Currency, termLeft, firstFuture int, capital int64,
) (eventOutcome, error) {
	if event.PrepaidAmount == nil {
		return eventOutcome{}, missingField("prepaid_amount")
	}
	if event.PrepayMode == nil {
		return eventOutcome{}, missingField("prepay_mode")
	}
	if !event.PrepayMode.Valid() {
		return eventOutcome{}, enumError("prepay_mode", string(*event.PrepayMode))
	}
	prepaid := event.PrepaidAmount.Minor()
	if prepaid <= 0 {
		return eventOutcome{}, newError(ErrInvalidValue,
			map[string]any{"field": "prepaid_amount", "value": prepaid},
			"prepaid amount must be positive")
	}
	if event.PrepaidAmount.Currency() != cur {
		return eventOutcome{}, newError(ErrCurrencyMismatch,
			map[string]any{"left": cur.Code(), "right": event.PrepaidAmount.Currency().Code()},
			"prepaid amount currency %s does not match loan currency %s",
			event.PrepaidAmount.Currency(), cur)
	}

	remaining := capital - prepaid
	if remaining < 0 {
		remaining = 0
	}
	if remaining == 0 {
		return eventOutcome{rate: sch.Rate, settled: true}, nil
	}

	term := termLeft
	if *event.PrepayMode == PrepaymentReduceTerm {
		// Keep the payment the amortization was using before the
		// prepayment, and shorten the term so the smaller capital is paid
		// off at least as fast.
		refInstallment := referenceInstallment(sch, firstFuture)
		n, err := termForFixedInstallment(remaining, sch.Rate, refInstallment, termLeft)
		if err != nil {
			return eventOutcome{}, err
		}
		term = n
	}
	// PrepaymentReduceInstallment keeps the term: ComputeSchedule derives a
	// smaller constant payment for the reduced capital.
	return eventOutcome{rate: sch.Rate, term: term, capital: remaining}, nil
}

// residualAtDate returns the outstanding capital, the remaining term
// (installments strictly after eventDate) and the index of the first such
// installment, using the schedule's installment list. Installments whose Due
// date is ≤ eventDate are considered already paid; the new schedule starts
// from the next installment.
func residualAtDate(sch Schedule, eventDate Date) (capitalMinor int64, termLeft, firstFuture int, err error) {
	// Find the index of the first installment whose Due > eventDate.
	firstFuture = len(sch.Installments) // assume all are past
	for i, inst := range sch.Installments {
		if inst.Due.After(eventDate) {
			firstFuture = i
			break
		}
	}

	if firstFuture == len(sch.Installments) {
		// event date is on or after the last installment due date.
		return 0, 0, 0, newError(ErrInvalidValue,
			map[string]any{"field": "event_date"},
			"event date %s is on or after the last installment", eventDate)
	}

	termLeft = len(sch.Installments) - firstFuture

	if firstFuture == 0 {
		// No installment has been paid yet — residual is the whole capital.
		capitalMinor = sumPrincipal(sch.Installments)
	} else {
		// Capital is the remaining balance after the last paid installment.
		capitalMinor = sch.Installments[firstFuture-1].Remaining.Minor()
	}
	return capitalMinor, termLeft, firstFuture, nil
}

// sumPrincipal returns the sum of all principal minor units.
func sumPrincipal(insts []Installment) int64 {
	var total int64
	for _, inst := range insts {
		total += inst.Principal.Minor()
	}
	return total
}

// referenceInstallment returns the constant payment (principal+interest, in
// minor units) in force in sch around index idx. The schedule's installments
// all share the same payment except the last, which absorbs the residual, so
// a non-last installment is preferred; if idx is the schedule's last
// installment the previous one is used instead.
func referenceInstallment(sch Schedule, idx int) int64 {
	last := len(sch.Installments) - 1
	if idx == last && idx > 0 {
		idx--
	}
	inst := sch.Installments[idx]
	return inst.Principal.Minor() + inst.Interest.Minor()
}

// termForFixedInstallment returns the smallest number of months in
// [1, maxTerm] whose French constant payment for capitalMinor at rate does
// not exceed targetInstallmentMinor. The payment is monotonically
// non-increasing in the term, so binary search applies. If even maxTerm
// months would need a larger payment (possible with a much higher new rate),
// maxTerm is returned: the caller never extends the term past what was left.
func termForFixedInstallment(capitalMinor int64, rate AnnualRate, targetInstallmentMinor int64, maxTerm int) (int, error) {
	if targetInstallmentMinor <= 0 {
		return 0, newError(ErrInvalidValue, nil, "target installment must be positive")
	}
	mRate := monthlyRate(rate.BasisPoints())
	lo, hi := 1, maxTerm
	atMax, err := frenchInstallmentAmount(capitalMinor, mRate, hi)
	if err != nil {
		return 0, err
	}
	if atMax > targetInstallmentMinor {
		return maxTerm, nil
	}
	for lo < hi {
		mid := (lo + hi) / 2
		r, err := frenchInstallmentAmount(capitalMinor, mRate, mid)
		if err != nil {
			return 0, err
		}
		if r <= targetInstallmentMinor {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, nil
}

// buildInstallment constructs an Installment from raw minor-unit values.
func buildInstallment(due Date, principalMinor, interestMinor, feesMinor, remainingMinor int64, cur Currency) (Installment, error) {
	principal, err := NewMoney(principalMinor, cur)
	if err != nil {
		return Installment{}, err
	}
	interest, err := NewMoney(interestMinor, cur)
	if err != nil {
		return Installment{}, err
	}
	fees, err := NewMoney(feesMinor, cur)
	if err != nil {
		return Installment{}, err
	}
	remaining, err := NewMoney(remainingMinor, cur)
	if err != nil {
		return Installment{}, err
	}
	return Installment{
		Due:       due,
		Principal: principal,
		Interest:  interest,
		Fees:      fees,
		Remaining: remaining,
	}, nil
}

// mulRatRound multiplies minor by r and rounds half-away-from-zero.
func mulRatRound(minor int64, r *big.Rat) (int64, error) {
	if r.Sign() == 0 {
		return 0, nil
	}
	product := new(big.Rat).Mul(new(big.Rat).SetInt64(minor), r)
	return RoundDiv(product.Num(), product.Denom())
}
