package domain

import (
	"math"
	"math/big"
	"slices"
)

// RunwayBalance is one account balance used by runway calculation.
type RunwayBalance struct {
	Kind    AccountKind
	Balance Money
}

// RunwayMovement is one historical movement used for monthly spending.
type RunwayMovement struct {
	Date   Date
	Amount Money
	Kind   TxKind
}

// RunwayResult is the runway KPI.
type RunwayResult struct {
	Liquidity                Money
	ReferenceMonthlySpending Money
	Months                   int
	Days                     int
	ZeroDate                 Date
}

// Runway computes runway from liquidity and historical monthly outflows.
func Runway(asOf Date, monthStartDay int, balances []RunwayBalance, movements []RunwayMovement) (RunwayResult, error) {
	base, err := runwayBaseCurrency(balances, movements)
	if err != nil {
		return RunwayResult{}, err
	}
	liquidity, err := runwayLiquidity(base, balances)
	if err != nil {
		return RunwayResult{}, err
	}
	spending, err := runwayReferenceSpending(asOf, monthStartDay, base, movements)
	if err != nil {
		return RunwayResult{}, err
	}
	return runwayDuration(asOf, liquidity, spending)
}

func runwayBaseCurrency(balances []RunwayBalance, movements []RunwayMovement) (Currency, error) {
	for _, item := range balances {
		if !item.Balance.Currency().IsZero() {
			return item.Balance.Currency(), nil
		}
	}
	for _, item := range movements {
		if !item.Amount.Currency().IsZero() {
			return item.Amount.Currency(), nil
		}
	}
	return Currency{}, missingField("currency")
}

func runwayLiquidity(base Currency, balances []RunwayBalance) (Money, error) {
	liquidity := Money{cur: base}
	debt := Money{cur: base}
	for _, item := range balances {
		if item.Balance.Currency() != base {
			return Money{}, newError(ErrCurrencyMismatch, map[string]any{
				"left":  item.Balance.Currency().Code(),
				"right": base.Code(),
			}, "currency mismatch: %s vs %s", item.Balance.Currency(), base)
		}
		switch item.Kind {
		case AccountCash, AccountChecking, AccountSavings:
			next, err := liquidity.Add(item.Balance)
			if err != nil {
				return Money{}, err
			}
			liquidity = next
		case AccountCreditCard:
			if item.Balance.Sign() < 0 {
				owed, err := item.Balance.Neg()
				if err != nil {
					return Money{}, err
				}
				next, err := debt.Add(owed)
				if err != nil {
					return Money{}, err
				}
				debt = next
			}
		case AccountInvestment, AccountLoan:
			// Excluded from liquidity by definition of the KPI.
		}
	}
	return liquidity.Sub(debt)
}

func runwayReferenceSpending(asOf Date, monthStartDay int, base Currency, movements []RunwayMovement) (Money, error) {
	window, err := lastCompleteMonthsWindow(asOf, monthStartDay, 12)
	if err != nil {
		return Money{}, err
	}
	monthly, active, err := monthlyOutflows(window, monthStartDay, base, movements)
	if err != nil {
		return Money{}, err
	}
	if active < 3 {
		return Money{}, newError(ErrNotEnoughData, map[string]any{"months": active}, "not enough complete months")
	}
	medianMinor, err := medianInt64(monthly)
	if err != nil {
		return Money{}, err
	}
	if medianMinor <= 0 {
		return Money{}, newError(ErrNotEnoughData, map[string]any{"median": medianMinor}, "reference spending is zero")
	}
	return NewMoney(medianMinor, base)
}

func lastCompleteMonthsWindow(asOf Date, monthStartDay, months int) (Period, error) {
	if asOf.IsZero() {
		return Period{}, missingField("date")
	}
	currentMonth, err := MonthPeriod(asOf, monthStartDay)
	if err != nil {
		return Period{}, err
	}
	end, err := currentMonth.From().AddDays(-1)
	if err != nil {
		return Period{}, err
	}
	start, err := currentMonth.From().AddMonths(-months)
	if err != nil {
		return Period{}, err
	}
	return NewPeriod(start, end)
}

func monthlyOutflows(window Period, monthStartDay int, base Currency, movements []RunwayMovement) ([]int64, int, error) {
	buckets, err := monthBuckets(window, monthStartDay)
	if err != nil {
		return nil, 0, err
	}
	active := map[Date]bool{}
	for _, movement := range movements {
		if movement.Date.IsZero() {
			return nil, 0, missingField("date")
		}
		if movement.Kind == KindTransfer || movement.Kind == KindInvestment {
			continue
		}
		if !window.Contains(movement.Date) {
			continue
		}
		if movement.Amount.Currency() != base {
			return nil, 0, newError(ErrCurrencyMismatch, map[string]any{
				"left": movement.Amount.Currency().Code(), "right": base.Code(),
			}, "currency mismatch: %s vs %s", movement.Amount.Currency(), base)
		}
		month, err := MonthPeriod(movement.Date, monthStartDay)
		if err != nil {
			return nil, 0, err
		}
		key := month.From()
		active[key] = true
		if movement.Amount.Sign() >= 0 {
			continue
		}
		buckets[key], err = addOutflow(buckets[key], movement.Amount)
		if err != nil {
			return nil, 0, err
		}
	}
	values := make([]int64, 0, len(active))
	for key := range active {
		values = append(values, buckets[key])
	}
	slices.Sort(values)
	return values, len(active), nil
}

func monthBuckets(window Period, monthStartDay int) (map[Date]int64, error) {
	buckets := map[Date]int64{}
	current, err := MonthPeriod(window.From(), monthStartDay)
	if err != nil {
		return nil, err
	}
	for {
		key := current.From()
		buckets[key] = 0
		if !current.To().Before(window.To()) {
			return buckets, nil
		}
		nextStart, err := current.To().AddDays(1)
		if err != nil {
			return nil, err
		}
		current, err = MonthPeriod(nextStart, monthStartDay)
		if err != nil {
			return nil, err
		}
	}
}

func addOutflow(total int64, amount Money) (int64, error) {
	absAmount, err := amount.Neg()
	if err != nil {
		return 0, err
	}
	sum := total + absAmount.Minor()
	if (total^sum)&(absAmount.Minor()^sum) < 0 {
		return 0, newError(ErrOverflow, nil, "outflow sum overflows")
	}
	return sum, nil
}

func medianInt64(values []int64) (int64, error) {
	if len(values) == 0 {
		return 0, newError(ErrNotEnoughData, nil, "not enough data")
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle], nil
	}
	sum := new(big.Int).Add(big.NewInt(sorted[middle-1]), big.NewInt(sorted[middle]))
	return RoundDiv(sum, big.NewInt(2))
}

func runwayDuration(asOf Date, liquidity, monthlySpending Money) (RunwayResult, error) {
	result := RunwayResult{
		Liquidity:                liquidity,
		ReferenceMonthlySpending: monthlySpending,
		ZeroDate:                 asOf,
	}
	if liquidity.Sign() <= 0 {
		return result, nil
	}
	num := new(big.Int).Mul(big.NewInt(liquidity.Minor()), big.NewInt(30))
	den := big.NewInt(monthlySpending.Minor())
	daysBig := new(big.Int).Quo(num, den)
	if !daysBig.IsInt64() {
		return RunwayResult{}, newError(ErrOverflow, nil, "runway exceeds int64 range")
	}
	days64 := daysBig.Int64()
	if days64 > math.MaxInt {
		return RunwayResult{}, newError(ErrOverflow, nil, "runway exceeds int range")
	}
	if days64 > maxShiftDays {
		return RunwayResult{}, newError(ErrOverflow, nil, "runway exceeds supported horizon")
	}
	result.Months = int(days64 / 30)
	result.Days = int(days64 % 30)
	zeroDate, err := asOf.AddDays(int(days64))
	if err != nil {
		return RunwayResult{}, err
	}
	result.ZeroDate = zeroDate
	return result, nil
}
