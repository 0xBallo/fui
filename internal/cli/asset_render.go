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
	Loan          string `json:"loan,omitempty"`
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
		Header:    []string{"NAME", "KIND", "CURRENCY", "VALUE DATE", "VALUE", "DEBT", "NET", "LOAN"},
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
			item.Loan,
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
		Loan:          item.Loan,
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

type assetShowDTO struct {
	assetItemDTO
	At      string              `json:"at"`
	History []assetHistoryEntry `json:"history"`
}

type assetHistoryEntry struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

type assetActionDTO struct {
	Action string `json:"action"`
	Asset  string `json:"asset"`
	Loan   string `json:"loan,omitempty"`
	Date   string `json:"date,omitempty"`
}

func writeAssetShow(w io.Writer, f render.Format, loc domain.Locale, color bool, v assetting.View) error {
	if f == render.JSON {
		dto := assetShowDTO{
			assetItemDTO: assetItemToDTO(v.Item), At: v.At.String(),
			History: make([]assetHistoryEntry, len(v.History)),
		}
		for i, h := range v.History {
			dto.History[i] = assetHistoryEntry{Date: h.Date.String(), Value: h.Value.Minor()}
		}
		return render.WriteJSON(w, dto)
	}
	rows := render.Rows{Header: []string{"DATE", "VALUE"}, MoneyCols: []int{1}, Body: make([][]string, 0, len(v.History))}
	for _, h := range v.History {
		val := render.FormatMoney(h.Value, loc)
		if f == render.CSV {
			val = render.FormatMoneyMachine(h.Value)
		}
		rows.Body = append(rows.Body, []string{h.Date.String(), val})
	}
	if f == render.CSV {
		return render.WriteCSV(w, rows)
	}
	loan := v.Loan
	if loan == "" {
		loan = "-"
	}
	_, err := fmt.Fprintf(w, "name: %s\nkind: %s\ncurrency: %s\npurchase: %s %s\nvalue at %s: %s (valued %s)\nloan: %s\ndebt: %s\nnet: %s\n",
		v.Asset.Name, v.Asset.Kind, v.Asset.PurchaseValue.Currency().Code(),
		v.Asset.PurchaseDate, render.FormatMoney(v.Asset.PurchaseValue, loc),
		v.At, render.FormatMoney(v.Latest.Value, loc), v.Latest.Date,
		loan, render.FormatMoney(v.Debt, loc), render.FormatMoney(v.Net, loc))
	if err != nil {
		return fmt.Errorf("write asset: %w", err)
	}
	if len(v.History) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "valuations:"); err != nil {
		return fmt.Errorf("write asset: %w", err)
	}
	return render.WriteTable(w, rows, color)
}

func writeAssetAction(w io.Writer, f render.Format, dto assetActionDTO) error {
	if f == render.JSON {
		return render.WriteJSON(w, dto)
	}
	rows := render.Rows{Header: []string{"ACTION", "ASSET", "LOAN", "DATE"}, Body: [][]string{{dto.Action, dto.Asset, dto.Loan, dto.Date}}}
	if f == render.CSV {
		return render.WriteCSV(w, rows)
	}
	_, err := fmt.Fprintf(w, "%s asset %s\n", dto.Action, dto.Asset)
	if err != nil {
		return fmt.Errorf("write asset: %w", err)
	}
	return nil
}
