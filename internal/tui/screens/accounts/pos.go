package accounts

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

type positionViewRow struct {
	id   string
	line investing.PositionLine
}

type positionState struct {
	rows  []positionViewRow
	table components.Table

	securityRef string
	priceField  components.Autocomplete
	dateField   components.Autocomplete
	form        components.Form
	notice      string
}

// SetPositionsUseCases wires the optional investing use cases for the positions panel.
func (m *Model) SetPositionsUseCases(uc PositionsUseCases) {
	m.positionsUC = uc
}

func (m *Model) keyBindings() []key.Binding {
	out := m.km.base()
	if m.positionsUC != nil {
		out = append(out, m.km.Price, m.km.Security, m.km.Op, m.km.Ops)
	}
	return out
}

func (m *Model) initPositions() {
	m.positions.table = components.NewTable([]components.Column{
		{Title: "Security", Width: 20, Align: lipgloss.Left},
		{Title: "Qty", Width: 12, Align: lipgloss.Right},
		{Title: "Avg cost", Width: 14, Align: lipgloss.Right},
		{Title: "Price", Width: 14, Align: lipgloss.Right},
		{Title: "Value", Width: 14, Align: lipgloss.Right},
		{Title: "P&L", Width: 14, Align: lipgloss.Right},
	}, m.th)
}

func (m *Model) resizePositions(w, h int) {
	rows := max(3, h/3)
	m.positions.table.SetHeight(rows)
	m.positions.table.SetWidth(w)
}

func (m *Model) positionsVisible() bool {
	return m.positionsUC != nil && m.opsAccount(m.detail.Account.Kind)
}

func (m *Model) loadDetailPositions(ctx context.Context, account domain.Account) ([]positionViewRow, error) {
	if m.positionsUC == nil || !m.opsAccount(account.Kind) {
		return nil, nil
	}
	lines, err := m.positionsUC.ListPositions(ctx, investing.PositionsRequest{Account: account.Name})
	if errors.Is(err, app.ErrModuleDisabled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows := make([]positionViewRow, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, positionViewRow{
			id:   strconv.FormatInt(int64(line.Security.ID), 10),
			line: line,
		})
	}
	return rows, nil
}

func (m *Model) setPositions(rows []positionViewRow) {
	m.positions.rows = rows
	tableRows := make([]components.Row, 0, len(rows))
	for _, row := range rows {
		pos := row.line.Position
		price, value, pnl := "-", "-", "-"
		if pos.HasPrice {
			price = pos.LastPrice.String()
			value = domain.FormatAmount(pos.MarketValue, m.loc)
			pnl = domain.FormatAmount(pos.Unrealized, m.loc)
		}
		tableRows = append(tableRows, components.Row{
			ID: row.id,
			Cells: []string{
				row.line.Security.Name,
				pos.Quantity.String(),
				domain.FormatAmount(pos.AvgCost, m.loc),
				price,
				value,
				pnl,
			},
		})
	}
	m.positions.table.SetRows(tableRows)
}

func (m *Model) viewPositions() string {
	if !m.positionsVisible() {
		return ""
	}
	out := m.th.Header.Render("Positions")
	if len(m.positions.rows) == 0 {
		return out + "\n" + m.th.Muted.Render("(no positions)")
	}
	return out + "\n" + m.positions.table.View()
}

func (m *Model) selectedPosition() (investing.PositionLine, bool) {
	row := m.positions.table.SelectedRow()
	if row == nil {
		return investing.PositionLine{}, false
	}
	for _, item := range m.positions.rows {
		if item.id == row.ID {
			return item.line, true
		}
	}
	return investing.PositionLine{}, false
}

func (m *Model) openPriceForm() {
	if !m.positionsVisible() {
		return
	}
	line, ok := m.selectedPosition()
	if !ok {
		return
	}
	m.positions.securityRef = line.Security.Name
	m.positions.notice = ""
	m.positions.priceField = components.NewAutocomplete("Price:", "e.g. 123.45", m.th)
	m.positions.dateField = components.NewAutocomplete("Date:", "today", m.th)
	m.positions.form = components.NewForm(&m.positions.priceField, &m.positions.dateField)
	m.positions.form.Focus()
	m.mode = modePriceForm
}

func (m *Model) handlePriceFormKey(msg tea.KeyPressMsg) (screens.Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDetail
		return m, nil
	case "shift+tab":
		m.positions.form.Prev()
		return m, nil
	case "enter":
		if m.positions.form.FocusedOpen() {
			m.positions.form.Update(msg)
			return m, nil
		}
		if !m.positions.form.AtLast() {
			m.positions.form.Next()
			return m, nil
		}
		return m, m.submitPrice()
	}
	m.positions.form.Update(msg)
	return m, nil
}

func (m *Model) submitPrice() tea.Cmd {
	if m.positionsUC == nil || m.positions.securityRef == "" {
		return nil
	}
	price := strings.TrimSpace(m.positions.priceField.Value())
	if price == "" {
		m.positions.notice = "Price is required."
		return nil
	}
	m.positions.notice = ""
	security := m.positions.securityRef
	date := strings.TrimSpace(m.positions.dateField.Value())
	account := m.detail.Account.Name
	return func() tea.Msg {
		plan, err := m.positionsUC.PlanSetPrice(context.Background(), investing.SetPriceRequest{
			Security: security,
			Date:     date,
			Price:    price,
		})
		if err != nil {
			return screens.ErrorMsg{Err: err}
		}
		if err := m.positionsUC.ApplySetPrice(context.Background(), plan); err != nil {
			return screens.ErrorMsg{Err: err}
		}
		return m.loadDetail(account)()
	}
}

func (m *Model) viewPriceForm() string {
	out := m.th.Title.Render("Set manual price") + "\n"
	out += m.th.Base.Render("Security: " + m.positions.securityRef)
	out += "\n\n" + m.positions.form.View()
	out += "\n\n" + m.th.Muted.Render("enter save · esc cancel")
	if m.positions.notice != "" {
		out += "\n" + m.th.Warning.Render(m.positions.notice)
	}
	return out
}
