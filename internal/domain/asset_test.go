package domain

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestAssetKind(t *testing.T) {
	got, err := ParseAssetKind("  PrOpErTy ")
	if err != nil {
		t.Fatal(err)
	}
	if got != AssetKindProperty {
		t.Fatalf("kind = %q, want %q", got, AssetKindProperty)
	}
	if _, err := ParseAssetKind(""); !errors.Is(err, ErrMissingField) {
		t.Fatalf("empty kind err = %v", err)
	}
	if _, err := ParseAssetKind("house"); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("invalid kind err = %v", err)
	}
}

func TestNewAsset(t *testing.T) {
	valid := Asset{
		ID:            1,
		Name:          "  Home  ",
		Kind:          AssetKindProperty,
		PurchaseDate:  date(t, 2026, 1, 1),
		PurchaseValue: money(t, 250_000_00, "EUR"),
		LoanID:        10,
	}
	got, err := NewAsset(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Home" {
		t.Fatalf("name = %q, want %q", got.Name, "Home")
	}

	cases := []struct {
		name  string
		asset Asset
		errIs error
	}{
		{
			name:  "missing name",
			asset: Asset{Kind: AssetKindProperty, PurchaseDate: valid.PurchaseDate, PurchaseValue: valid.PurchaseValue},
			errIs: ErrMissingField,
		},
		{
			name:  "missing kind",
			asset: Asset{Name: "Home", PurchaseDate: valid.PurchaseDate, PurchaseValue: valid.PurchaseValue},
			errIs: ErrMissingField,
		},
		{
			name:  "invalid kind",
			asset: Asset{Name: "Home", Kind: "boat", PurchaseDate: valid.PurchaseDate, PurchaseValue: valid.PurchaseValue},
			errIs: ErrInvalidValue,
		},
		{
			name:  "missing purchase date",
			asset: Asset{Name: "Home", Kind: AssetKindProperty, PurchaseValue: valid.PurchaseValue},
			errIs: ErrMissingField,
		},
		{
			name:  "missing purchase value",
			asset: Asset{Name: "Home", Kind: AssetKindProperty, PurchaseDate: valid.PurchaseDate},
			errIs: ErrMissingField,
		},
		{
			name:  "negative purchase value",
			asset: Asset{Name: "Home", Kind: AssetKindProperty, PurchaseDate: valid.PurchaseDate, PurchaseValue: money(t, -1, "EUR")},
			errIs: ErrInvalidValue,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAsset(tc.asset)
			if !errors.Is(err, tc.errIs) {
				t.Fatalf("err = %v, want errors.Is(%v)", err, tc.errIs)
			}
		})
	}
}

func TestLatestValuation(t *testing.T) {
	asset := Asset{
		ID:            42,
		Name:          "Home",
		Kind:          AssetKindProperty,
		PurchaseDate:  date(t, 2020, 1, 1),
		PurchaseValue: money(t, 120_000_00, "EUR"),
	}
	valuations := []Valuation{
		{AssetID: 42, Date: date(t, 2022, 12, 31), Value: money(t, 140_000_00, "EUR")},
		{AssetID: 42, Date: date(t, 2024, 6, 1), Value: money(t, 160_000_00, "EUR")},
		{AssetID: 42, Date: date(t, 2025, 1, 1), Value: money(t, 170_000_00, "EUR")},
	}

	got, err := LatestValuation(asset, valuations, date(t, 2024, 12, 31))
	if err != nil {
		t.Fatal(err)
	}
	if got.Date != date(t, 2024, 6, 1) || got.Value != money(t, 160_000_00, "EUR") {
		t.Fatalf("got = %+v", got)
	}

	fallback, err := LatestValuation(asset, valuations, date(t, 2020, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Date != asset.PurchaseDate || fallback.Value != asset.PurchaseValue {
		t.Fatalf("fallback = %+v, want purchase", fallback)
	}
}

func TestLatestValuationErrors(t *testing.T) {
	asset := Asset{
		ID:            1,
		Name:          "Car",
		Kind:          AssetKindVehicle,
		PurchaseDate:  date(t, 2020, 1, 1),
		PurchaseValue: money(t, 10_000_00, "EUR"),
	}
	if _, err := LatestValuation(asset, nil, Date{}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing date err = %v", err)
	}
	_, err := LatestValuation(asset, []Valuation{
		{AssetID: 1, Date: date(t, 2021, 1, 1), Value: money(t, 1, "USD")},
	}, date(t, 2021, 1, 1))
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch err = %v", err)
	}
}

func TestNewValuation(t *testing.T) {
	valid, err := NewValuation(Valuation{
		AssetID: 1,
		Date:    date(t, 2026, 1, 1),
		Value:   money(t, 1, "EUR"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if valid.AssetID != 1 {
		t.Fatalf("asset id = %d", valid.AssetID)
	}
	if _, err := NewValuation(Valuation{Value: money(t, 1, "EUR")}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing date err = %v", err)
	}
	if _, err := NewValuation(Valuation{Date: date(t, 2026, 1, 1)}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("missing value err = %v", err)
	}
	if _, err := NewValuation(Valuation{Date: date(t, 2026, 1, 1), Value: money(t, -1, "EUR")}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("negative value err = %v", err)
	}
}

func TestLatestValuationSkipsDifferentAssetID(t *testing.T) {
	asset := Asset{
		ID:            2,
		Name:          "Bike",
		Kind:          AssetKindVehicle,
		PurchaseDate:  date(t, 2020, 1, 1),
		PurchaseValue: money(t, 2_000_00, "EUR"),
	}
	got, err := LatestValuation(asset, []Valuation{
		{AssetID: 1, Date: date(t, 2021, 1, 1), Value: money(t, 9_999_00, "EUR")},
		{AssetID: 2, Date: date(t, 2021, 6, 1), Value: money(t, 1_500_00, "EUR")},
	}, date(t, 2021, 12, 31))
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != money(t, 1_500_00, "EUR") {
		t.Fatalf("value = %v", got.Value)
	}
}

func TestLatestValuationUsesLatestDateOnOrBeforeProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		purchaseDate := date(t, 2020, 1, 1)
		asOfOffset := rapid.IntRange(0, 2000).Draw(t, "as_of_offset")
		asOf, err := purchaseDate.AddDays(asOfOffset)
		if err != nil {
			t.Fatal(err)
		}
		asset := Asset{
			ID:            7,
			Name:          "Asset",
			Kind:          AssetKindOther,
			PurchaseDate:  purchaseDate,
			PurchaseValue: money(t, rapid.Int64Range(1, 1_000_000_00).Draw(t, "purchase"), "EUR"),
		}
		n := rapid.IntRange(0, 30).Draw(t, "n")
		valuations := make([]Valuation, 0, n)
		wantDate := asset.PurchaseDate
		wantValue := asset.PurchaseValue
		for range n {
			offset := rapid.IntRange(0, 2500).Draw(t, "offset")
			d, err := purchaseDate.AddDays(offset)
			if err != nil {
				t.Fatal(err)
			}
			value := money(t, rapid.Int64Range(0, 2_000_000_00).Draw(t, "value"), "EUR")
			valuation := Valuation{AssetID: 7, Date: d, Value: value}
			valuations = append(valuations, valuation)
			if !d.After(asOf) && wantDate.Before(d) {
				wantDate = d
				wantValue = value
			}
		}
		got, err := LatestValuation(asset, valuations, asOf)
		if err != nil {
			t.Fatal(err)
		}
		if got.Date != wantDate || got.Value != wantValue {
			t.Fatalf("got = (%v,%v), want = (%v,%v)", got.Date, got.Value, wantDate, wantValue)
		}
	})
}
