package cli

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
)

type kpiNetWorthAmountDTO struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
}

type kpiNetWorthDTO struct {
	AsOf      string                   `json:"as_of"`
	Currency  string                   `json:"currency"`
	Total     int64                    `json:"total"`
	Lines     []render.NetWorthLineDTO `json:"lines"`
	Liquidity kpiNetWorthAmountDTO     `json:"liquidity"`
	Positions *kpiNetWorthAmountDTO    `json:"positions,omitempty"`
	Assets    *kpiNetWorthAmountDTO    `json:"assets,omitempty"`
	Debts     *kpiNetWorthAmountDTO    `json:"debts,omitempty"`
	// AssetLines itemizes each asset in its own currency (not base-converted); it is
	// informational and not part of the totals.
	AssetLines []kpiNetWorthAssetDTO `json:"asset_lines,omitempty"`
}

type kpiNetWorthAssetDTO struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Value    int64  `json:"value"`
	Debt     int64  `json:"debt"`
	Net      int64  `json:"net"`
	Loan     string `json:"loan"`
}

func netWorthAssetLines(items []assetting.Item) []kpiNetWorthAssetDTO {
	if items == nil {
		return nil
	}
	out := make([]kpiNetWorthAssetDTO, len(items))
	for i, it := range items {
		out[i] = kpiNetWorthAssetDTO{
			Name: it.Asset.Name, Currency: it.Latest.Value.Currency().Code(),
			Value: it.Latest.Value.Minor(), Debt: it.Debt.Minor(), Net: it.Net.Minor(), Loan: it.Loan,
		}
	}
	return out
}

func writeKPINetWorth(
	w io.Writer,
	f render.Format,
	loc domain.Locale,
	color bool,
	nw app.NetWorth,
	point reporting.NetWorthPoint,
	items []assetting.Item,
) error {
	components, err := netWorthComponents(nw, point)
	if err != nil {
		return err
	}
	if f == render.JSON {
		return render.WriteJSON(w, kpiNetWorthDTO{
			AsOf: point.Date, Currency: components.Total.Currency().Code(), Total: components.Total.Minor(), Lines: netWorthLines(nw),
			Liquidity: components.Liquidity, Positions: components.Positions, Assets: components.Assets, Debts: components.Debts,
			AssetLines: netWorthAssetLines(items),
		})
	}
	rows, err := kpiNetWorthRows(nw, components, loc, f == render.CSV)
	if err != nil {
		return err
	}
	appendAssetRows(&rows, items, loc, f == render.CSV)
	if f == render.CSV {
		return render.WriteCSV(w, rows)
	}
	return render.WriteTable(w, rows, color)
}

// appendAssetRows adds one "ASSET <name>" row per asset after the totals, with three extra
// VALUE/DEBT/NET columns that exist only when there is at least one asset; the existing
// columns and rows are otherwise untouched.
func appendAssetRows(rows *render.Rows, items []assetting.Item, loc domain.Locale, csv bool) {
	if len(items) == 0 {
		return
	}
	fmtMoney := func(m domain.Money) string {
		if csv {
			return render.FormatMoneyMachine(m)
		}
		return render.FormatMoney(m, loc)
	}
	rows.Header = append(rows.Header, "ASSET VALUE", "ASSET DEBT", "ASSET NET")
	rows.MoneyCols = append(rows.MoneyCols, 4, 5, 6)
	for i := range rows.Body {
		rows.Body[i] = append(rows.Body[i], "", "", "")
	}
	for _, it := range items {
		rows.Body = append(rows.Body, []string{
			"ASSET " + it.Asset.Name, it.Latest.Value.Currency().Code(), "", "",
			fmtMoney(it.Latest.Value), fmtMoney(it.Debt), fmtMoney(it.Net),
		})
	}
}

type netWorthComponentSet struct {
	Total     domain.Money
	Liquidity kpiNetWorthAmountDTO
	Positions *kpiNetWorthAmountDTO
	Assets    *kpiNetWorthAmountDTO
	Debts     *kpiNetWorthAmountDTO
}

func netWorthComponents(nw app.NetWorth, point reporting.NetWorthPoint) (netWorthComponentSet, error) {
	total := nw.Total
	var err error
	if point.Positions != nil {
		var positions domain.Money
		positions, err = moneyFromAmount(*point.Positions)
		if err != nil {
			return netWorthComponentSet{}, err
		}
		if total, err = total.Add(positions); err != nil {
			return netWorthComponentSet{}, err
		}
	}
	if point.Assets != nil {
		var assets domain.Money
		assets, err = moneyFromAmount(*point.Assets)
		if err != nil {
			return netWorthComponentSet{}, err
		}
		if total, err = total.Add(assets); err != nil {
			return netWorthComponentSet{}, err
		}
	}
	total, err = domain.NewMoney(total.Minor(), total.Currency())
	if err != nil {
		return netWorthComponentSet{}, err
	}
	out := netWorthComponentSet{
		Total: total,
		Liquidity: kpiNetWorthAmountDTO{
			Minor: point.Liquidity.Minor, Currency: point.Liquidity.Currency,
		},
	}
	if point.Positions != nil {
		out.Positions = &kpiNetWorthAmountDTO{Minor: point.Positions.Minor, Currency: point.Positions.Currency}
	}
	if point.Assets != nil {
		out.Assets = &kpiNetWorthAmountDTO{Minor: point.Assets.Minor, Currency: point.Assets.Currency}
	}
	if point.Debts != nil {
		out.Debts = &kpiNetWorthAmountDTO{Minor: point.Debts.Minor, Currency: point.Debts.Currency}
	}
	return out, nil
}

func kpiNetWorthRows(nw app.NetWorth, c netWorthComponentSet, loc domain.Locale, csv bool) (render.Rows, error) {
	rows := render.Rows{Header: []string{"ACCOUNT", "CURRENCY", "BALANCE", "CONVERTED"}, MoneyCols: []int{2, 3}}
	for _, l := range nw.Lines {
		bal := render.FormatMoney(l.Balance, loc)
		conv := render.FormatMoney(l.Converted, loc)
		if csv {
			bal = render.FormatMoneyMachine(l.Balance)
			conv = render.FormatMoneyMachine(l.Converted)
		}
		rows.Body = append(rows.Body, []string{l.Account.Name, l.Account.Currency.Code(), bal, conv})
	}
	if err := appendComponentRows(&rows, loc, csv, c); err != nil {
		return render.Rows{}, err
	}
	return rows, nil
}

func appendComponentRows(rows *render.Rows, loc domain.Locale, csv bool, c netWorthComponentSet) error {
	add := func(label string, dto *kpiNetWorthAmountDTO) error {
		if dto == nil {
			return nil
		}
		money, err := moneyFromAmount(reporting.Amount{Minor: dto.Minor, Currency: dto.Currency})
		if err != nil {
			return err
		}
		amount := render.FormatMoney(money, loc)
		if csv {
			amount = render.FormatMoneyMachine(money)
		}
		rows.Body = append(rows.Body, []string{label, dto.Currency, "", amount})
		return nil
	}
	if err := add("LIQUIDITY", &c.Liquidity); err != nil {
		return err
	}
	if err := add("POSITIONS", c.Positions); err != nil {
		return err
	}
	if err := add("ASSETS", c.Assets); err != nil {
		return err
	}
	if err := add("DEBTS", c.Debts); err != nil {
		return err
	}
	total := render.FormatMoney(c.Total, loc)
	if csv {
		total = render.FormatMoneyMachine(c.Total)
	}
	rows.Body = append(rows.Body, []string{"TOTAL", c.Total.Currency().Code(), "", total})
	return nil
}

func netWorthLines(nw app.NetWorth) []render.NetWorthLineDTO {
	out := make([]render.NetWorthLineDTO, len(nw.Lines))
	for i, line := range nw.Lines {
		out[i] = render.NetWorthLineDTO{
			Name: line.Account.Name, Currency: line.Account.Currency.Code(),
			Balance: line.Balance.Minor(), Converted: line.Converted.Minor(),
		}
	}
	return out
}

func moneyFromAmount(a reporting.Amount) (domain.Money, error) {
	currency, err := domain.ParseCurrency(a.Currency)
	if err != nil {
		return domain.Money{}, fmt.Errorf("parse currency %q: %w", a.Currency, err)
	}
	money, err := domain.NewMoney(a.Minor, currency)
	if err != nil {
		return domain.Money{}, err
	}
	return money, nil
}
