package render

import (
	"fmt"
	"io"
	"sort"

	"github.com/0xballo/fui/internal/app/budgeting"
	"github.com/0xballo/fui/internal/domain"
)

// BudgetTargetDTO is the --json shape of one budget target: exactly one of category_id or tag_id
// is set, matching domain.BudgetTarget.
type BudgetTargetDTO struct {
	CategoryID int64 `json:"category_id,omitempty"`
	TagID      int64 `json:"tag_id,omitempty"`
}

// BudgetDTO is the --json shape of one budget.
type BudgetDTO struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Mode      string            `json:"mode"`
	Targets   []BudgetTargetDTO `json:"targets"`
	Amount    int64             `json:"amount"`
	Currency  string            `json:"currency"`
	Carry     bool              `json:"carry"`
	Overrides map[string]int64  `json:"overrides,omitempty"`
	WarnAt    int               `json:"warn_at"`
	OverAt    int               `json:"over_at"`
}

func budgetTargetDTOs(ts []domain.BudgetTarget) []BudgetTargetDTO {
	out := make([]BudgetTargetDTO, len(ts))
	for i, t := range ts {
		out[i] = BudgetTargetDTO{CategoryID: int64(t.CategoryID), TagID: int64(t.TagID)}
	}
	return out
}

func budgetOverridesDTO(overrides map[domain.MonthKey]domain.Money) map[string]int64 {
	if len(overrides) == 0 {
		return nil
	}
	out := make(map[string]int64, len(overrides))
	for k, v := range overrides {
		out[monthKeyString(k)] = v.Minor()
	}
	return out
}

func monthKeyString(k domain.MonthKey) string {
	return fmt.Sprintf("%04d-%02d", k.Year, k.Month)
}

func budgetDTO(b domain.Budget) BudgetDTO {
	return BudgetDTO{
		ID: int64(b.ID), Name: b.Name, Mode: string(b.Mode), Targets: budgetTargetDTOs(b.Targets),
		Amount: b.Amount.Minor(), Currency: b.Amount.Currency().Code(), Carry: b.Carry,
		Overrides: budgetOverridesDTO(b.Overrides), WarnAt: b.Thresholds.Warn, OverAt: b.Thresholds.Over,
	}
}

func budgetRows(budgets []domain.Budget, loc domain.Locale) Rows {
	t := Rows{Header: []string{"NAME", "MODE", "AMOUNT", "CARRY"}, MoneyCols: []int{2}}
	for _, b := range budgets {
		t.Body = append(t.Body, []string{b.Name, string(b.Mode), FormatMoney(b.Amount, loc), fmt.Sprint(b.Carry)})
	}
	return t
}

// WriteBudgetList renders every budget.
func WriteBudgetList(w io.Writer, f Format, loc domain.Locale, budgets []domain.Budget) error {
	if f == JSON {
		dtos := make([]BudgetDTO, len(budgets))
		for i, b := range budgets {
			dtos[i] = budgetDTO(b)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, budgetRows(budgets, loc))
	}
	return WriteTable(w, budgetRows(budgets, loc), false)
}

// WriteBudget renders one budget.
func WriteBudget(w io.Writer, f Format, loc domain.Locale, b domain.Budget) error {
	if f == JSON {
		return WriteJSON(w, budgetDTO(b))
	}
	return WriteBudgetList(w, f, loc, []domain.Budget{b})
}

// stateLabel turns a BudgetState into the text label the spec requires alongside (not instead
// of) color, so a non-color terminal or --csv still shows the threshold state.
func stateLabel(s domain.BudgetState) string {
	switch s {
	case domain.BudgetOver:
		return "OVER"
	case domain.BudgetWarning:
		return "WARNING"
	case domain.BudgetOK:
		return "OK"
	default:
		return "OK"
	}
}

func envelopeState(m domain.EnvelopeMonth) string {
	if m.Overspend.Sign() > 0 {
		return "OVER"
	}
	return "OK"
}

// CapStatusDTO is the --json shape of one cap evaluated for the requested month.
type CapStatusDTO struct {
	Name      string `json:"name"`
	Amount    int64  `json:"amount"`
	CarryIn   int64  `json:"carry_in"`
	Available int64  `json:"available"`
	Spent     int64  `json:"spent"`
	Planned   int64  `json:"planned"`
	Remaining int64  `json:"remaining"`
	Percent   int    `json:"percent"`
	State     string `json:"state"`
}

func capStatusDTO(l budgeting.CapLine) CapStatusDTO {
	return CapStatusDTO{
		Name: l.Budget.Name, Amount: l.Amount.Minor(), CarryIn: l.CarryIn.Minor(), Available: l.Available.Minor(),
		Spent: l.Spent.Minor(), Planned: l.Planned.Minor(), Remaining: l.Remaining.Minor(), Percent: l.Percent,
		State: stateLabel(l.State),
	}
}

// EnvelopeStatusDTO is the --json shape of one envelope's ledger for the requested month.
type EnvelopeStatusDTO struct {
	Name      string `json:"name"`
	CarryIn   int64  `json:"carry_in"`
	Assigned  int64  `json:"assigned"`
	MovesIn   int64  `json:"moves_in"`
	MovesOut  int64  `json:"moves_out"`
	Spent     int64  `json:"spent"`
	Planned   int64  `json:"planned"`
	Balance   int64  `json:"balance"`
	Overspend int64  `json:"overspend"`
	State     string `json:"state"`
}

func envelopeStatusDTO(l budgeting.EnvelopeLine) EnvelopeStatusDTO {
	return EnvelopeStatusDTO{
		Name: l.Budget.Name, CarryIn: l.CarryIn.Minor(), Assigned: l.Assigned.Minor(), MovesIn: l.MovesIn.Minor(),
		MovesOut: l.MovesOut.Minor(), Spent: l.Spent.Minor(), Planned: l.Planned.Minor(), Balance: l.Balance.Minor(),
		Overspend: l.Overspend.Minor(), State: envelopeState(l.EnvelopeMonth),
	}
}

// ToAssignDTO is the --json shape of the envelope-mode "Da assegnare" summary.
type ToAssignDTO struct {
	Liquidity int64 `json:"liquidity"`
	Envelopes int64 `json:"envelopes"`
	ToAssign  int64 `json:"to_assign"`
}

func toAssignDTO(u domain.Unassigned) ToAssignDTO {
	return ToAssignDTO{Liquidity: u.Liquidity.Minor(), Envelopes: u.Envelopes.Minor(), ToAssign: u.ToAssign.Minor()}
}

// BudgetStatusDTO is the --json shape of `fui budget status`.
type BudgetStatusDTO struct {
	From      string              `json:"from"`
	To        string              `json:"to"`
	Mode      string              `json:"mode"`
	Caps      []CapStatusDTO      `json:"caps,omitempty"`
	Envelopes []EnvelopeStatusDTO `json:"envelopes,omitempty"`
	ToAssign  *ToAssignDTO        `json:"to_assign,omitempty"`
	Uncovered int64               `json:"uncovered"`
}

func budgetStatusDTO(st budgeting.Status) BudgetStatusDTO {
	out := BudgetStatusDTO{
		From: st.Period.From().String(), To: st.Period.To().String(), Mode: string(st.Mode),
		Uncovered: st.Uncovered.Minor(),
	}
	for _, l := range st.Caps {
		out.Caps = append(out.Caps, capStatusDTO(l))
	}
	for _, l := range st.Envelopes {
		out.Envelopes = append(out.Envelopes, envelopeStatusDTO(l))
	}
	if st.Mode == domain.BudgetEnvelope {
		ta := toAssignDTO(st.ToAssign)
		out.ToAssign = &ta
	}
	return out
}

func capStatusRows(caps []budgeting.CapLine, loc domain.Locale) Rows {
	t := Rows{Header: []string{"NAME", "AVAILABLE", "SPENT", "PLANNED", "REMAINING", "%", "STATE"}, MoneyCols: []int{1, 2, 3, 4}}
	sorted := append([]budgeting.CapLine(nil), caps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Budget.Name < sorted[j].Budget.Name })
	for _, l := range sorted {
		t.Body = append(t.Body, []string{
			l.Budget.Name, FormatMoney(l.Available, loc), FormatMoney(l.Spent, loc), FormatMoney(l.Planned, loc),
			FormatMoney(l.Remaining, loc), fmt.Sprint(l.Percent), stateLabel(l.State),
		})
	}
	return t
}

func envelopeStatusRows(envs []budgeting.EnvelopeLine, loc domain.Locale) Rows {
	t := Rows{
		Header:    []string{"NAME", "CARRY_IN", "ASSIGNED", "MOVES_IN", "MOVES_OUT", "SPENT", "PLANNED", "BALANCE", "STATE"},
		MoneyCols: []int{1, 2, 3, 4, 5, 6, 7},
	}
	sorted := append([]budgeting.EnvelopeLine(nil), envs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Budget.Name < sorted[j].Budget.Name })
	for _, l := range sorted {
		t.Body = append(t.Body, []string{
			l.Budget.Name, FormatMoney(l.CarryIn, loc), FormatMoney(l.Assigned, loc), FormatMoney(l.MovesIn, loc),
			FormatMoney(l.MovesOut, loc), FormatMoney(l.Spent, loc), FormatMoney(l.Planned, loc),
			FormatMoney(l.Balance, loc), envelopeState(l.EnvelopeMonth),
		})
	}
	return t
}

// WriteBudgetStatus renders the state of every budget for one month: a cap table in cap mode, an
// envelope table plus "Da assegnare" in envelope mode, and the spending covered by no budget in
// both. The table always carries a text STATE label, never color alone.
func WriteBudgetStatus(w io.Writer, f Format, loc domain.Locale, color bool, st budgeting.Status) error {
	if f == JSON {
		return WriteJSON(w, budgetStatusDTO(st))
	}
	rows := capStatusRows(st.Caps, loc)
	if st.Mode == domain.BudgetEnvelope {
		rows = envelopeStatusRows(st.Envelopes, loc)
	}
	if f == CSV {
		return WriteCSV(w, rows)
	}
	if err := WriteTable(w, rows, color); err != nil {
		return err
	}
	return writeBudgetStatusFooter(w, loc, st)
}

func writeBudgetStatusFooter(w io.Writer, loc domain.Locale, st budgeting.Status) error {
	if st.Mode == domain.BudgetEnvelope {
		if _, err := fmt.Fprintf(w, "To assign: %s\n", FormatMoney(st.ToAssign.ToAssign, loc)); err != nil {
			return fmt.Errorf("write to-assign: %w", err)
		}
	}
	if _, err := fmt.Fprintf(w, "Uncovered: %s\n", FormatMoney(st.Uncovered, loc)); err != nil {
		return fmt.Errorf("write uncovered: %w", err)
	}
	return nil
}

// BudgetMoveDTO is the --json shape of an envelope assignment or move.
type BudgetMoveDTO struct {
	Date   string `json:"date"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Amount int64  `json:"amount"`
}

// WriteBudgetMove renders an applied envelope move: from or to is empty for the unassigned pool.
func WriteBudgetMove(w io.Writer, f Format, loc domain.Locale, date domain.Date, from, to string, amount domain.Money) error {
	if f == JSON {
		return WriteJSON(w, BudgetMoveDTO{Date: date.String(), From: from, To: to, Amount: amount.Minor()})
	}
	t := Rows{Header: []string{"DATE", "FROM", "TO", "AMOUNT"}, MoneyCols: []int{3}}
	t.Body = [][]string{{date.String(), from, to, FormatMoney(amount, loc)}}
	if f == CSV {
		return WriteCSV(w, t)
	}
	return WriteTable(w, t, false)
}

// WriteBudgetCopyResult reports how many budget updates and envelope moves a copy applied.
func WriteBudgetCopyResult(w io.Writer, f Format, updates, moves int) error {
	if f == JSON {
		return WriteJSON(w, struct {
			Updates int `json:"updates"`
			Moves   int `json:"moves"`
		}{updates, moves})
	}
	_, err := fmt.Fprintf(w, "Updated %d budget(s), %d move(s).\n", updates, moves)
	if err != nil {
		return fmt.Errorf("write copy result: %w", err)
	}
	return nil
}
