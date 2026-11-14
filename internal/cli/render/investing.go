package render

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
)

// SecurityDTO is the --json shape of one security.
type SecurityDTO struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Ticker        string `json:"ticker,omitempty"`
	ISIN          string `json:"isin,omitempty"`
	Kind          string `json:"kind"`
	Currency      string `json:"currency"`
	QuantityScale int    `json:"quantity_scale"`
}

func securityDTO(s domain.Security) SecurityDTO {
	return SecurityDTO{
		ID:            int64(s.ID),
		Name:          s.Name,
		Ticker:        s.Ticker,
		ISIN:          s.ISIN,
		Kind:          string(s.Kind),
		Currency:      s.Currency.Code(),
		QuantityScale: s.QuantityScale,
	}
}

func securityRows(secs []domain.Security) Rows {
	t := Rows{Header: []string{"ID", "NAME", "TICKER", "ISIN", "KIND", "CURRENCY", "QUANTITY_SCALE"}}
	for _, s := range secs {
		t.Body = append(t.Body, []string{
			fmt.Sprint(s.ID), s.Name, s.Ticker, s.ISIN, string(s.Kind), s.Currency.Code(), fmt.Sprint(s.QuantityScale),
		})
	}
	return t
}

// WriteSecurityList renders a list of securities.
func WriteSecurityList(w io.Writer, f Format, secs []domain.Security) error {
	if f == JSON {
		dtos := make([]SecurityDTO, len(secs))
		for i, s := range secs {
			dtos[i] = securityDTO(s)
		}
		return WriteJSON(w, dtos)
	}
	rows := securityRows(secs)
	if f == CSV {
		return WriteCSV(w, rows)
	}
	return WriteTable(w, rows, false)
}

// WriteSecurity renders one security.
func WriteSecurity(w io.Writer, f Format, sec domain.Security) error {
	if f == JSON {
		return WriteJSON(w, securityDTO(sec))
	}
	return WriteSecurityList(w, f, []domain.Security{sec})
}

// SecurityPriceView is one manual security price rendered by the CLI.
type SecurityPriceView struct {
	SecurityID int64
	Date       domain.Date
	Price      domain.Money
}

// SecurityPriceDTO is the --json shape of one manual security price.
type SecurityPriceDTO struct {
	SecurityID int64  `json:"security_id"`
	Date       string `json:"date"`
	Price      int64  `json:"price"`
	Currency   string `json:"currency"`
}

func securityPriceDTO(v SecurityPriceView) SecurityPriceDTO {
	return SecurityPriceDTO{
		SecurityID: v.SecurityID, Date: v.Date.String(), Price: v.Price.Minor(), Currency: v.Price.Currency().Code(),
	}
}

func securityPriceRows(lines []SecurityPriceView, loc domain.Locale, machine bool) Rows {
	t := Rows{Header: []string{"SECURITY_ID", "DATE", "PRICE", "CURRENCY"}, MoneyCols: []int{2}}
	for _, line := range lines {
		price := FormatMoney(line.Price, loc)
		if machine {
			price = FormatMoneyMachine(line.Price)
		}
		date := FormatDate(line.Date, loc)
		if machine {
			date = FormatDateMachine(line.Date)
		}
		t.Body = append(t.Body, []string{
			fmt.Sprint(line.SecurityID), date, price, line.Price.Currency().Code(),
		})
	}
	return t
}

// WriteSecurityPrice renders one manual security price.
func WriteSecurityPrice(w io.Writer, f Format, loc domain.Locale, line SecurityPriceView) error {
	if f == JSON {
		return WriteJSON(w, securityPriceDTO(line))
	}
	if f == CSV {
		return WriteCSV(w, securityPriceRows([]SecurityPriceView{line}, loc, true))
	}
	return WriteTable(w, securityPriceRows([]SecurityPriceView{line}, loc, false), false)
}

// SecurityOpDTO is the --json shape of one security operation.
type SecurityOpDTO struct {
	ID         int64  `json:"id"`
	Date       string `json:"date"`
	Kind       string `json:"kind"`
	AccountID  int64  `json:"account_id"`
	SecurityID int64  `json:"security_id"`
	Quantity   string `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	Fees       int64  `json:"fees"`
	Currency   string `json:"currency"`
	CashTxID   int64  `json:"cash_tx_id,omitempty"`
}

func securityOpDTO(op domain.SecurityOp) SecurityOpDTO {
	return SecurityOpDTO{
		ID: int64(op.ID), Date: op.Date.String(), Kind: string(op.Kind), AccountID: int64(op.Account),
		SecurityID: int64(op.Security), Quantity: op.Quantity.String(), UnitPrice: op.UnitPrice.Minor(),
		Fees: op.Fees.Minor(), Currency: op.UnitPrice.Currency().Code(), CashTxID: int64(op.CashTx),
	}
}

func securityOpRows(ops []domain.SecurityOp, loc domain.Locale, machine bool) Rows {
	t := Rows{
		Header:    []string{"ID", "DATE", "KIND", "ACCOUNT_ID", "SECURITY_ID", "QUANTITY", "UNIT_PRICE", "FEES", "CURRENCY", "CASH_TX_ID"},
		MoneyCols: []int{6, 7},
	}
	for _, op := range ops {
		unitPrice := FormatMoney(op.UnitPrice, loc)
		fees := FormatMoney(op.Fees, loc)
		date := FormatDate(op.Date, loc)
		if machine {
			unitPrice = FormatMoneyMachine(op.UnitPrice)
			fees = FormatMoneyMachine(op.Fees)
			date = FormatDateMachine(op.Date)
		}
		t.Body = append(t.Body, []string{
			fmt.Sprint(op.ID), date, string(op.Kind), fmt.Sprint(op.Account), fmt.Sprint(op.Security), op.Quantity.String(),
			unitPrice, fees, op.UnitPrice.Currency().Code(), fmt.Sprint(op.CashTx),
		})
	}
	return t
}

// WriteSecurityOp renders one security operation.
func WriteSecurityOp(w io.Writer, f Format, loc domain.Locale, op domain.SecurityOp) error {
	if f == JSON {
		return WriteJSON(w, securityOpDTO(op))
	}
	if f == CSV {
		return WriteCSV(w, securityOpRows([]domain.SecurityOp{op}, loc, true))
	}
	return WriteTable(w, securityOpRows([]domain.SecurityOp{op}, loc, false), false)
}

// PositionLineDTO is the --json shape of one computed position line.
type PositionLineDTO struct {
	Account       string `json:"account"`
	AccountID     int64  `json:"account_id"`
	Security      string `json:"security"`
	SecurityID    int64  `json:"security_id"`
	Kind          string `json:"kind"`
	Currency      string `json:"currency"`
	AsOf          string `json:"as_of"`
	Quantity      string `json:"quantity"`
	CostBasis     int64  `json:"cost_basis"`
	AvgCost       int64  `json:"avg_cost"`
	Realized      int64  `json:"realized"`
	Income        int64  `json:"income"`
	HasPrice      bool   `json:"has_price"`
	LastPrice     int64  `json:"last_price,omitempty"`
	PriceDate     string `json:"price_date,omitempty"`
	MarketValue   int64  `json:"market_value,omitempty"`
	Unrealized    int64  `json:"unrealized,omitempty"`
	Value         int64  `json:"value,omitempty"`
	ValueCurrency string `json:"value_currency,omitempty"`
}

func positionLineDTO(line investing.PositionLine) PositionLineDTO {
	dto := PositionLineDTO{
		Account:    line.Account.Name,
		AccountID:  int64(line.Account.ID),
		Security:   line.Security.Name,
		SecurityID: int64(line.Security.ID),
		Kind:       string(line.Security.Kind),
		Currency:   line.Security.Currency.Code(),
		AsOf:       line.Position.AsOf.String(),
		Quantity:   line.Position.Quantity.String(),
		CostBasis:  line.Position.CostBasis.Minor(),
		AvgCost:    line.Position.AvgCost.Minor(),
		Realized:   line.Position.Realized.Minor(),
		Income:     line.Position.Income.Minor(),
		HasPrice:   line.Position.HasPrice,
	}
	if line.Position.HasPrice {
		dto.LastPrice = line.Position.LastPrice.Minor()
		dto.PriceDate = line.Position.PriceDate.String()
		dto.MarketValue = line.Position.MarketValue.Minor()
		dto.Unrealized = line.Position.Unrealized.Minor()
		dto.Value = line.Value.Minor()
		dto.ValueCurrency = line.Value.Currency().Code()
	}
	return dto
}

func positionRows(lines []investing.PositionLine, loc domain.Locale, machine bool) Rows {
	t := Rows{
		Header:    []string{"ACCOUNT", "SECURITY", "KIND", "AS_OF", "QUANTITY", "COST_BASIS", "AVG_COST", "REALIZED", "INCOME", "LAST_PRICE", "PRICE_DATE", "MARKET_VALUE", "UNREALIZED", "VALUE"},
		MoneyCols: []int{5, 6, 7, 8, 9, 11, 12, 13},
	}
	for _, line := range lines {
		pos := line.Position
		asOf := FormatDate(pos.AsOf, loc)
		costBasis := FormatMoney(pos.CostBasis, loc)
		avgCost := FormatMoney(pos.AvgCost, loc)
		realized := FormatMoney(pos.Realized, loc)
		income := FormatMoney(pos.Income, loc)
		lastPrice := "-"
		priceDate := "-"
		marketValue := "-"
		unrealized := "-"
		value := "-"
		if pos.HasPrice {
			lastPrice = FormatMoney(pos.LastPrice, loc)
			priceDate = FormatDate(pos.PriceDate, loc)
			marketValue = FormatMoney(pos.MarketValue, loc)
			unrealized = FormatMoney(pos.Unrealized, loc)
			value = FormatMoney(line.Value, loc)
		}
		if machine {
			asOf = FormatDateMachine(pos.AsOf)
			costBasis = FormatMoneyMachine(pos.CostBasis)
			avgCost = FormatMoneyMachine(pos.AvgCost)
			realized = FormatMoneyMachine(pos.Realized)
			income = FormatMoneyMachine(pos.Income)
			lastPrice = ""
			priceDate = ""
			marketValue = ""
			unrealized = ""
			value = ""
			if pos.HasPrice {
				lastPrice = FormatMoneyMachine(pos.LastPrice)
				priceDate = FormatDateMachine(pos.PriceDate)
				marketValue = FormatMoneyMachine(pos.MarketValue)
				unrealized = FormatMoneyMachine(pos.Unrealized)
				value = FormatMoneyMachine(line.Value)
			}
		}
		t.Body = append(t.Body, []string{
			line.Account.Name, line.Security.Name, string(line.Security.Kind), asOf, pos.Quantity.String(),
			costBasis, avgCost, realized, income, lastPrice, priceDate, marketValue, unrealized, value,
		})
	}
	return t
}

// WritePositionList renders investment positions.
func WritePositionList(w io.Writer, f Format, loc domain.Locale, color bool, lines []investing.PositionLine) error {
	if f == JSON {
		dtos := make([]PositionLineDTO, len(lines))
		for i, line := range lines {
			dtos[i] = positionLineDTO(line)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, positionRows(lines, loc, true))
	}
	return WriteTable(w, positionRows(lines, loc, false), color)
}
