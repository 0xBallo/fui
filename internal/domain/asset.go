package domain

// AssetKind classifies a non-financial asset.
type AssetKind string

// Known asset kinds.
const (
	AssetKindProperty AssetKind = "property"
	AssetKindVehicle  AssetKind = "vehicle"
	AssetKindOther    AssetKind = "other"
)

// Valid reports whether k is a known AssetKind.
func (k AssetKind) Valid() bool {
	switch k {
	case AssetKindProperty, AssetKindVehicle, AssetKindOther:
		return true
	}
	return false
}

// ParseAssetKind parses an asset kind (case-insensitive).
func ParseAssetKind(s string) (AssetKind, error) {
	return parseEnum("asset kind", s, AssetKind.Valid)
}

// AssetID identifies an Asset.
type AssetID int64

// Asset is a non-financial owned good contributing to net worth.
type Asset struct {
	ID            AssetID
	Name          string
	Kind          AssetKind
	PurchaseDate  Date
	PurchaseValue Money
	LoanID        LoanID // optional linked loan (zero means none)
}

// NewAsset validates a and returns it with the name trimmed.
func NewAsset(a Asset) (Asset, error) {
	name, err := requireName("name", a.Name)
	if err != nil {
		return Asset{}, err
	}
	a.Name = name
	if !a.Kind.Valid() {
		return Asset{}, enumError("kind", string(a.Kind))
	}
	if a.PurchaseDate.IsZero() {
		return Asset{}, missingField("purchase_date")
	}
	if a.PurchaseValue.Currency().IsZero() {
		return Asset{}, missingField("purchase_value")
	}
	if a.PurchaseValue.Sign() < 0 {
		return Asset{}, newError(ErrInvalidValue,
			map[string]any{"field": "purchase_value", "value": a.PurchaseValue.Minor()},
			"purchase_value must be non-negative")
	}
	return a, nil
}

// Valuation is one manual asset valuation on a date.
type Valuation struct {
	AssetID AssetID
	Date    Date
	Value   Money
}

// NewValuation validates v.
func NewValuation(v Valuation) (Valuation, error) {
	if v.Date.IsZero() {
		return Valuation{}, missingField("date")
	}
	if v.Value.Currency().IsZero() {
		return Valuation{}, missingField("value")
	}
	if v.Value.Sign() < 0 {
		return Valuation{}, newError(ErrInvalidValue,
			map[string]any{"field": "value", "value": v.Value.Minor()},
			"value must be non-negative")
	}
	return v, nil
}

// LatestValuation returns the latest valuation on or before at.
// If none exists, it falls back to asset purchase date and value.
func LatestValuation(asset Asset, valuations []Valuation, at Date) (Valuation, error) {
	if at.IsZero() {
		return Valuation{}, missingField("date")
	}
	validAsset, err := NewAsset(asset)
	if err != nil {
		return Valuation{}, err
	}
	latest := Valuation{
		AssetID: validAsset.ID,
		Date:    validAsset.PurchaseDate,
		Value:   validAsset.PurchaseValue,
	}
	for _, valuation := range valuations {
		validValuation, err := NewValuation(valuation)
		if err != nil {
			return Valuation{}, err
		}
		if validValuation.Value.Currency() != validAsset.PurchaseValue.Currency() {
			return Valuation{}, newError(ErrCurrencyMismatch, map[string]any{
				"left":  validValuation.Value.Currency().Code(),
				"right": validAsset.PurchaseValue.Currency().Code(),
			}, "currency mismatch: %s vs %s", validValuation.Value.Currency(), validAsset.PurchaseValue.Currency())
		}
		if validAsset.ID != 0 && validValuation.AssetID != 0 && validValuation.AssetID != validAsset.ID {
			continue
		}
		if validValuation.Date.After(at) {
			continue
		}
		if latest.Date.Before(validValuation.Date) {
			latest = Valuation{
				AssetID: validAsset.ID,
				Date:    validValuation.Date,
				Value:   validValuation.Value,
			}
		}
	}
	return latest, nil
}
