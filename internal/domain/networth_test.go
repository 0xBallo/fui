package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestNetWorth(t *testing.T) {
	input := NetWorthInput{
		AsOf:      date(t, 2026, 12, 31),
		Liquidity: money(t, 50_000_00, "EUR"),
		Positions: money(t, 20_000_00, "EUR"),
		Assets: []NetWorthAssetInput{
			{
				Asset: Asset{
					ID:            1,
					Name:          "Home",
					Kind:          AssetKindProperty,
					PurchaseDate:  date(t, 2020, 1, 1),
					PurchaseValue: money(t, 100_000_00, "EUR"),
					LoanID:        10,
				},
				Valuations: []Valuation{
					{AssetID: 1, Date: date(t, 2024, 1, 1), Value: money(t, 150_000_00, "EUR")},
				},
			},
		},
		LoanDebts: map[LoanID]Money{
			10: money(t, 30_000_00, "EUR"),
		},
		Modules: NetWorthModules{
			Investments: true,
			Assets:      true,
			Loans:       true,
		},
	}

	got, err := NetWorth(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Liquidity.Minor() != 50_000_00 {
		t.Fatalf("liquidity = %d", got.Liquidity.Minor())
	}
	if got.Positions.Minor() != 20_000_00 {
		t.Fatalf("positions = %d", got.Positions.Minor())
	}
	if got.Assets.Minor() != 150_000_00 {
		t.Fatalf("assets = %d", got.Assets.Minor())
	}
	if got.Debts.Minor() != -30_000_00 {
		t.Fatalf("debts = %d", got.Debts.Minor())
	}
	if got.Total.Minor() != 190_000_00 {
		t.Fatalf("total = %d", got.Total.Minor())
	}
	if len(got.AssetValues) != 1 {
		t.Fatalf("asset lines = %d", len(got.AssetValues))
	}
	line := got.AssetValues[0]
	if line.GrossValue.Minor() != 150_000_00 || line.DebtValue.Minor() != 30_000_00 || line.NetValue.Minor() != 120_000_00 {
		t.Fatalf("asset line = %+v", line)
	}
}

func TestNetWorthTotalIsSumOfComponentsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := randomNetWorthInput(t)
		got, err := NetWorth(input)
		if err != nil {
			t.Fatal(err)
		}
		sum, err := got.Liquidity.Add(got.Positions)
		if err != nil {
			t.Fatal(err)
		}
		sum, err = sum.Add(got.Assets)
		if err != nil {
			t.Fatal(err)
		}
		sum, err = sum.Add(got.Debts)
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != sum {
			t.Fatalf("total = %v, sum = %v", got.Total, sum)
		}
	})
}

func TestNetWorthDisablingModuleRemovesExactlyItsComponentProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := randomNetWorthInput(t)
		full, err := NetWorth(input)
		if err != nil {
			t.Fatal(err)
		}

		noInvest := input
		noInvest.Modules.Investments = false
		gotNoInvest, err := NetWorth(noInvest)
		if err != nil {
			t.Fatal(err)
		}
		diffInvest, err := full.Total.Sub(gotNoInvest.Total)
		if err != nil {
			t.Fatal(err)
		}
		if diffInvest != full.Positions {
			t.Fatalf("invest diff = %v, want %v", diffInvest, full.Positions)
		}

		noAssets := input
		noAssets.Modules.Assets = false
		gotNoAssets, err := NetWorth(noAssets)
		if err != nil {
			t.Fatal(err)
		}
		diffAssets, err := full.Total.Sub(gotNoAssets.Total)
		if err != nil {
			t.Fatal(err)
		}
		if diffAssets != full.Assets {
			t.Fatalf("assets diff = %v, want %v", diffAssets, full.Assets)
		}

		noLoans := input
		noLoans.Modules.Loans = false
		gotNoLoans, err := NetWorth(noLoans)
		if err != nil {
			t.Fatal(err)
		}
		diffLoans, err := full.Total.Sub(gotNoLoans.Total)
		if err != nil {
			t.Fatal(err)
		}
		if diffLoans != full.Debts {
			t.Fatalf("loans diff = %v, want %v", diffLoans, full.Debts)
		}
	})
}

func TestNetWorthUsesLastValuationOnDateProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		purchaseDate := date(t, 2020, 1, 1)
		asOfOffset := rapid.IntRange(0, 2000).Draw(t, "as_of_offset")
		asOf, err := purchaseDate.AddDays(asOfOffset)
		if err != nil {
			t.Fatal(err)
		}
		purchaseValue := money(t, rapid.Int64Range(1, 1_000_000_00).Draw(t, "purchase"), "EUR")
		asset := Asset{
			ID:            1,
			Name:          "A",
			Kind:          AssetKindOther,
			PurchaseDate:  purchaseDate,
			PurchaseValue: purchaseValue,
		}
		valuations := make([]Valuation, 0, 25)
		wantDate := purchaseDate
		wantValue := purchaseValue
		count := rapid.IntRange(0, 25).Draw(t, "count")
		for range count {
			offset := rapid.IntRange(0, 2500).Draw(t, "offset")
			d, err := purchaseDate.AddDays(offset)
			if err != nil {
				t.Fatal(err)
			}
			v := money(t, rapid.Int64Range(0, 2_000_000_00).Draw(t, "v"), "EUR")
			valuations = append(valuations, Valuation{AssetID: 1, Date: d, Value: v})
			if !d.After(asOf) && wantDate.Before(d) {
				wantDate = d
				wantValue = v
			}
		}
		got, err := NetWorth(NetWorthInput{
			AsOf:      asOf,
			Liquidity: money(t, 1, "EUR"),
			Assets: []NetWorthAssetInput{
				{Asset: asset, Valuations: valuations},
			},
			Modules: NetWorthModules{Assets: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Assets != wantValue {
			t.Fatalf("assets value = %v, want %v", got.Assets, wantValue)
		}
		if got.AssetValues[0].ValuationDate != wantDate {
			t.Fatalf("valuation date = %v, want %v", got.AssetValues[0].ValuationDate, wantDate)
		}
	})
}

func TestNetWorthErrorsAndDisabledModules(t *testing.T) {
	base := NetWorthInput{
		AsOf:      date(t, 2026, 12, 31),
		Liquidity: money(t, 1, "EUR"),
		Positions: money(t, 1, "EUR"),
		Assets: []NetWorthAssetInput{
			{
				Asset: Asset{
					ID:            1,
					Name:          "A",
					Kind:          AssetKindOther,
					PurchaseDate:  date(t, 2020, 1, 1),
					PurchaseValue: money(t, 10, "EUR"),
					LoanID:        2,
				},
				Valuations: []Valuation{
					{AssetID: 1, Date: date(t, 2026, 1, 1), Value: money(t, 11, "EUR")},
				},
			},
		},
		LoanDebts: map[LoanID]Money{2: money(t, 3, "EUR")},
		Modules:   NetWorthModules{Investments: true, Assets: true, Loans: true},
	}

	_, err := NetWorth(NetWorthInput{})
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing liquidity err = %v", err)
	}

	invalidPos := base
	invalidPos.Positions = money(t, 1, "USD")
	_, err = NetWorth(invalidPos)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("positions mismatch err = %v", err)
	}

	ignoredPos := base
	ignoredPos.Modules.Investments = false
	ignoredPos.Positions = money(t, 1, "USD")
	if _, err := NetWorth(ignoredPos); err != nil {
		t.Fatalf("disabled investments should ignore positions: %v", err)
	}

	missingAsOf := base
	missingAsOf.AsOf = Date{}
	_, err = NetWorth(missingAsOf)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing as_of err = %v", err)
	}

	invalidDebt := base
	invalidDebt.LoanDebts = map[LoanID]Money{2: money(t, -1, "EUR")}
	_, err = NetWorth(invalidDebt)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("negative debt err = %v", err)
	}

	invalidDebtCurrency := base
	invalidDebtCurrency.LoanDebts = map[LoanID]Money{2: money(t, 1, "USD")}
	_, err = NetWorth(invalidDebtCurrency)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("debt currency err = %v", err)
	}

	invalidAssetCurrency := base
	invalidAssetCurrency.Assets[0].Valuations = []Valuation{
		{AssetID: 1, Date: date(t, 2026, 1, 1), Value: money(t, 11, "USD")},
	}
	_, err = NetWorth(invalidAssetCurrency)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("asset currency err = %v", err)
	}

	invalidZeroDebt := base
	invalidZeroDebt.LoanDebts = map[LoanID]Money{2: {}}
	_, err = NetWorth(invalidZeroDebt)
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing debt money err = %v", err)
	}
}

func randomNetWorthInput(t *rapid.T) NetWorthInput {
	t.Helper()
	assetID := AssetID(1)
	loanID := LoanID(1)
	purchaseDate := date(t, 2020, 1, 1)
	asOf, err := purchaseDate.AddDays(rapid.IntRange(0, 1000).Draw(t, "as_of"))
	if err != nil {
		t.Fatal(err)
	}
	purchase := money(t, rapid.Int64Range(1, 300_000_00).Draw(t, "purchase"), "EUR")
	valuationDate, err := purchaseDate.AddDays(rapid.IntRange(0, 1000).Draw(t, "valuation_date"))
	if err != nil {
		t.Fatal(err)
	}
	valuationValue := money(t, rapid.Int64Range(1, 500_000_00).Draw(t, "valuation_value"), "EUR")
	loanDebt := money(t, rapid.Int64Range(0, 200_000_00).Draw(t, "loan_debt"), "EUR")
	return NetWorthInput{
		AsOf:      asOf,
		Liquidity: money(t, rapid.Int64Range(-100_000_00, 500_000_00).Draw(t, "liquidity"), "EUR"),
		Positions: money(t, rapid.Int64Range(0, 300_000_00).Draw(t, "positions"), "EUR"),
		Assets: []NetWorthAssetInput{
			{
				Asset: Asset{
					ID:            assetID,
					Name:          "A",
					Kind:          AssetKindProperty,
					PurchaseDate:  purchaseDate,
					PurchaseValue: purchase,
					LoanID:        loanID,
				},
				Valuations: []Valuation{
					{AssetID: assetID, Date: valuationDate, Value: valuationValue},
				},
			},
		},
		LoanDebts: map[LoanID]Money{loanID: loanDebt},
		Modules: NetWorthModules{
			Investments: true,
			Assets:      true,
			Loans:       true,
		},
	}
}
