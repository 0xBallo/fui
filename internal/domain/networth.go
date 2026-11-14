package domain

// NetWorthModules contains module-activation flags affecting the composition.
type NetWorthModules struct {
	Investments bool
	Assets      bool
	Loans       bool
}

// NetWorthAssetInput carries one asset and its valuation history.
type NetWorthAssetInput struct {
	Asset      Asset
	Valuations []Valuation
}

// NetWorthInput is the input for net-worth composition in base currency.
type NetWorthInput struct {
	AsOf      Date
	Liquidity Money
	Positions Money
	Assets    []NetWorthAssetInput
	LoanDebts map[LoanID]Money // residual debts as positive values
	Modules   NetWorthModules
}

// NetWorthAssetValue is one asset line with gross and debt-adjusted values.
type NetWorthAssetValue struct {
	AssetID       AssetID
	ValuationDate Date
	GrossValue    Money
	DebtValue     Money // positive residual debt linked to this asset
	NetValue      Money
}

// NetWorthResult contains net-worth components and the final total.
type NetWorthResult struct {
	Liquidity   Money
	Positions   Money
	Assets      Money
	Debts       Money // negative component
	Total       Money
	AssetValues []NetWorthAssetValue
}

// NetWorth composes net worth as liquidity + positions + assets - debts,
// considering only active modules.
func NetWorth(in NetWorthInput) (NetWorthResult, error) {
	if in.Liquidity.Currency().IsZero() {
		return NetWorthResult{}, missingField("liquidity")
	}
	base := in.Liquidity.Currency()
	zero := Money{cur: base}
	result := NetWorthResult{
		Liquidity: in.Liquidity,
		Positions: zero,
		Assets:    zero,
		Debts:     zero,
		Total:     zero,
	}
	positions, err := netWorthPositions(base, in.Positions, in.Modules.Investments)
	if err != nil {
		return NetWorthResult{}, err
	}
	result.Positions = positions
	loanDebts, debtByLoan, err := sumLoanDebts(base, in.LoanDebts, in.Modules.Loans)
	if err != nil {
		return NetWorthResult{}, err
	}
	result.Debts = loanDebts
	assetsValue, assetValues, err := netWorthAssets(base, in.AsOf, in.Assets, debtByLoan, in.Modules.Assets, in.Modules.Loans)
	if err != nil {
		return NetWorthResult{}, err
	}
	result.Assets = assetsValue
	result.AssetValues = assetValues
	result.Total, err = netWorthTotal(result)
	if err != nil {
		return NetWorthResult{}, err
	}
	return result, nil
}

func netWorthPositions(base Currency, positions Money, enabled bool) (Money, error) {
	zero := Money{cur: base}
	if !enabled {
		return zero, nil
	}
	if positions.Currency().IsZero() {
		return zero, nil
	}
	if positions.Currency() != base {
		return Money{}, newError(ErrCurrencyMismatch, map[string]any{
			"left":  positions.Currency().Code(),
			"right": base.Code(),
		}, "currency mismatch: %s vs %s", positions.Currency(), base)
	}
	return positions, nil
}

func netWorthAssets(
	base Currency,
	asOf Date,
	input []NetWorthAssetInput,
	debtByLoan map[LoanID]Money,
	assetsEnabled, loansEnabled bool,
) (Money, []NetWorthAssetValue, error) {
	zero := Money{cur: base}
	if !assetsEnabled {
		return zero, nil, nil
	}
	if asOf.IsZero() {
		return Money{}, nil, missingField("date")
	}
	values := make([]NetWorthAssetValue, 0, len(input))
	total := zero
	for _, inputAsset := range input {
		line, err := netWorthAssetLine(base, asOf, inputAsset, debtByLoan, loansEnabled)
		if err != nil {
			return Money{}, nil, err
		}
		values = append(values, line)
		total, err = total.Add(line.GrossValue)
		if err != nil {
			return Money{}, nil, err
		}
	}
	return total, values, nil
}

func netWorthAssetLine(
	base Currency,
	asOf Date,
	input NetWorthAssetInput,
	debtByLoan map[LoanID]Money,
	loansEnabled bool,
) (NetWorthAssetValue, error) {
	valuation, err := LatestValuation(input.Asset, input.Valuations, asOf)
	if err != nil {
		return NetWorthAssetValue{}, err
	}
	if valuation.Value.Currency() != base {
		return NetWorthAssetValue{}, newError(ErrCurrencyMismatch, map[string]any{
			"left":  valuation.Value.Currency().Code(),
			"right": base.Code(),
		}, "currency mismatch: %s vs %s", valuation.Value.Currency(), base)
	}
	debt := Money{cur: base}
	if loansEnabled && input.Asset.LoanID != 0 {
		if linkedDebt, ok := debtByLoan[input.Asset.LoanID]; ok {
			debt = linkedDebt
		}
	}
	net, err := valuation.Value.Sub(debt)
	if err != nil {
		return NetWorthAssetValue{}, err
	}
	return NetWorthAssetValue{
		AssetID:       input.Asset.ID,
		ValuationDate: valuation.Date,
		GrossValue:    valuation.Value,
		DebtValue:     debt,
		NetValue:      net,
	}, nil
}

func netWorthTotal(result NetWorthResult) (Money, error) {
	total, err := result.Liquidity.Add(result.Positions)
	if err != nil {
		return Money{}, err
	}
	total, err = total.Add(result.Assets)
	if err != nil {
		return Money{}, err
	}
	return total.Add(result.Debts)
}

func sumLoanDebts(base Currency, debts map[LoanID]Money, enabled bool) (Money, map[LoanID]Money, error) {
	zero := Money{cur: base}
	if !enabled {
		return zero, map[LoanID]Money{}, nil
	}
	sum := zero
	byLoan := make(map[LoanID]Money, len(debts))
	for loanID, debt := range debts {
		if debt.Currency().IsZero() {
			return Money{}, nil, missingField("loan_debt")
		}
		if debt.Currency() != base {
			return Money{}, nil, newError(ErrCurrencyMismatch, map[string]any{
				"left":  debt.Currency().Code(),
				"right": base.Code(),
			}, "currency mismatch: %s vs %s", debt.Currency(), base)
		}
		if debt.Sign() < 0 {
			return Money{}, nil, newError(ErrInvalidValue, map[string]any{
				"field": "loan_debt", "loan_id": int64(loanID), "value": debt.Minor(),
			}, "loan debt must be non-negative")
		}
		next, err := sum.Add(debt)
		if err != nil {
			return Money{}, nil, err
		}
		sum = next
		byLoan[loanID] = debt
	}
	negative, err := sum.Neg()
	if err != nil {
		return Money{}, nil, err
	}
	return negative, byLoan, nil
}
