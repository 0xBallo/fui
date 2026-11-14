package cli

import (
	"fmt"
	"io"
	"strconv"

	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

type assetDTO struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	PurchaseDate  string `json:"purchase_date"`
	PurchaseValue int64  `json:"purchase_value"`
	Currency      string `json:"currency"`
	LoanID        *int64 `json:"loan_id,omitempty"`
}

type assetItemDTO struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Currency      string `json:"currency"`
	PurchaseDate  string `json:"purchase_date"`
	PurchaseValue int64  `json:"purchase_value"`
	ValueDate     string `json:"value_date"`
	Value         int64  `json:"value"`
	Debt          int64  `json:"debt"`
	Net           int64  `json:"net"`
	LoanID        *int64 `json:"loan_id,omitempty"`
}

type assetValuationDTO struct {
	Asset     string `json:"asset"`
	Date      string `json:"date"`
	Value     int64  `json:"value"`
	Currency  string `json:"currency"`
	Overwrite bool   `json:"overwrite"`
	Previous  *int64 `json:"previous,omitempty"`
}

func writeAsset(w io.Writer, f render.Format, loc domain.Locale, color bool, a domain.Asset) error {
	dto := assetToDTO(a)
	if f == render.JSON {
		return render.WriteJSON(w, dto)
	}
	rows := render.Rows{
		Header:    []string{"ID", "NAME", "KIND", "PURCHASE DATE", "PURCHASE VALUE", "CURRENCY", "LOAN ID"},
		MoneyCols: []int{4},
		Body: [][]string{{
			strconv.FormatInt(dto.ID, 10),
			dto.Name,
			dto.Kind,
			dto.PurchaseDate,
			render.FormatMoney(a.PurchaseValue, loc),
			dto.Currency,
			optionalID(dto.LoanID),
		}},
	}
	if f == render.CSV {
		rows.Body[0][4] = render.FormatMoneyMachine(a.PurchaseValue)
		return render.WriteCSV(w, rows)
	}
	return render.WriteTable(w, rows, color)
}

func writeAssetList(w io.Writer, f render.Format, loc domain.Locale, color bool, items []assetting.Item) error {
	if f == render.JSON {
		out := make([]assetItemDTO, len(items))
		for i, item := range items {
			out[i] = assetItemToDTO(item)
		}
		return render.WriteJSON(w, out)
	}
	rows := render.Rows{
		Header:    []string{"NAME", "KIND", "CURRENCY", "VALUE DATE", "VALUE", "DEBT", "NET"},
		MoneyCols: []int{4, 5, 6},
		Body:      make([][]string, 0, len(items)),
	}
	for _, item := range items {
		row := []string{
			item.Asset.Name,
			string(item.Asset.Kind),
			item.Asset.PurchaseValue.Currency().Code(),
			item.Latest.Date.String(),
			render.FormatMoney(item.Latest.Value, loc),
			render.FormatMoney(item.Debt, loc),
			render.FormatMoney(item.Net, loc),
		}
		if f == render.CSV {
			row[4] = render.FormatMoneyMachine(item.Latest.Value)
			row[5] = render.FormatMoneyMachine(item.Debt)
			row[6] = render.FormatMoneyMachine(item.Net)
		}
		rows.Body = append(rows.Body, row)
	}
	if f == render.CSV {
		return render.WriteCSV(w, rows)
	}
	return render.WriteTable(w, rows, color)
}

func writeAssetValuation(w io.Writer, f render.Format, loc domain.Locale, color bool, p assetting.ValuePlan) error {
	dto := assetValuationDTO{
		Asset: p.Asset.Name, Date: p.Valuation.Date.String(), Value: p.Valuation.Value.Minor(),
		Currency: p.Valuation.Value.Currency().Code(), Overwrite: p.Overwrite,
	}
	if p.Overwrite {
		prev := p.Previous.Minor()
		dto.Previous = &prev
	}
	if f == render.JSON {
		return render.WriteJSON(w, dto)
	}
	rows := render.Rows{
		Header:    []string{"ASSET", "DATE", "VALUE", "CURRENCY", "OVERWRITE", "PREVIOUS"},
		MoneyCols: []int{2, 5},
		Body: [][]string{{
			dto.Asset,
			dto.Date,
			render.FormatMoney(p.Valuation.Value, loc),
			dto.Currency,
			strconv.FormatBool(dto.Overwrite),
			"",
		}},
	}
	if dto.Previous != nil {
		previous, err := domain.NewMoney(*dto.Previous, p.Valuation.Value.Currency())
		if err != nil {
			return fmt.Errorf("previous valuation: %w", err)
		}
		rows.Body[0][5] = render.FormatMoney(previous, loc)
	}
	if f == render.CSV {
		rows.Body[0][2] = render.FormatMoneyMachine(p.Valuation.Value)
		if dto.Previous != nil {
			previous, err := domain.NewMoney(*dto.Previous, p.Valuation.Value.Currency())
			if err != nil {
				return fmt.Errorf("previous valuation: %w", err)
			}
			rows.Body[0][5] = render.FormatMoneyMachine(previous)
		}
		return render.WriteCSV(w, rows)
	}
	return render.WriteTable(w, rows, color)
}

func assetToDTO(a domain.Asset) assetDTO {
	out := assetDTO{
		ID:            int64(a.ID),
		Name:          a.Name,
		Kind:          string(a.Kind),
		PurchaseDate:  a.PurchaseDate.String(),
		PurchaseValue: a.PurchaseValue.Minor(),
		Currency:      a.PurchaseValue.Currency().Code(),
	}
	if a.LoanID != 0 {
		loanID := int64(a.LoanID)
		out.LoanID = &loanID
	}
	return out
}

func assetItemToDTO(item assetting.Item) assetItemDTO {
	out := assetItemDTO{
		ID:            int64(item.Asset.ID),
		Name:          item.Asset.Name,
		Kind:          string(item.Asset.Kind),
		Currency:      item.Asset.PurchaseValue.Currency().Code(),
		PurchaseDate:  item.Asset.PurchaseDate.String(),
		PurchaseValue: item.Asset.PurchaseValue.Minor(),
		ValueDate:     item.Latest.Date.String(),
		Value:         item.Latest.Value.Minor(),
		Debt:          item.Debt.Minor(),
		Net:           item.Net.Minor(),
	}
	if item.Asset.LoanID != 0 {
		loanID := int64(item.Asset.LoanID)
		out.LoanID = &loanID
	}
	return out
}

func optionalID(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}
