package report

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/tui/components"
	"github.com/0xballo/fui/internal/tui/screens"
)

// defaultRateSpreadBP is the distance of the pessimistic and optimistic rates from a lone rate.
const defaultRateSpreadBP = 200

type simField struct {
	key, label, placeholder string
	suggestions             []string
}

var (
	yesNo        = []string{"no", "yes"}
	compoundings = []string{"monthly", "annual"}
	frequencies  = []string{"monthly", "quarterly", "yearly"}
)

var (
	fInitial     = simField{"initial", "Initial capital:", "e.g. 10000", nil}
	fPositions   = simField{"positions", "Start from current positions:", "no", yesNo}
	fYears       = simField{"years", "Years:", "e.g. 10", nil}
	fCompounding = simField{"compounding", "Compounding:", "monthly", compoundings}
	fRates       = simField{"rates", "Rates % (one, or low,base,high):", "e.g. 3,5,7", nil}
	fCost        = simField{"cost", "Annual cost %:", "0", nil}
	fTax         = simField{"tax", "Tax on gain %:", "0", nil}
	fInflation   = simField{"inflation", "Inflation %:", "0", nil}
	fContrib     = simField{"contribution", "Contribution:", "e.g. 200", nil}
	fFrequency   = simField{"frequency", "Frequency:", "monthly", frequencies}
	fGrowth      = simField{"growth", "Contribution growth %:", "0", nil}
	fMonths      = simField{"months", "Months:", "e.g. 12", nil}
	fOneOff      = simField{"oneoff", "One-off (date:amount; ...):", "2027-03-01:-5000", nil}
	fChange      = simField{"change", "Monthly change (date:amount; ...):", "2027-06-01:300", nil}
	fStopIncome  = simField{"stopincome", "Stop income (date; ...):", "2027-01-01", nil}
	fLoan        = simField{"loan", "New loan (date:capital:rate:years; ...):", "2027-02-01:20000:4.5:5", nil}
	fAccounts    = simField{"accounts", "Accounts (comma separated, empty = all):", "all", nil}
)

func simFields(kind string) []simField {
	switch kind {
	case simulating.KindPAC:
		return []simField{fContrib, fFrequency, fInitial, fPositions, fYears, fCompounding, fRates, fGrowth, fCost, fTax, fInflation}
	case simulating.KindCashFlow:
		return []simField{fMonths, fOneOff, fChange, fStopIncome, fLoan, fAccounts}
	}
	return []simField{fInitial, fPositions, fYears, fCompounding, fRates, fCost, fTax, fInflation}
}

func defaultSimValues() map[string]string {
	return map[string]string{
		"initial": "10000", "positions": "no", "years": "10", "compounding": "monthly", "rates": "3,5,7",
		"cost": "0", "tax": "0", "inflation": "0", "contribution": "200", "frequency": "monthly",
		"growth": "0", "months": "12",
	}
}

func (m *Model) openSimForm() {
	fields := simFields(simKinds[m.sim.kindIdx])
	m.sim.fields = make([]components.Autocomplete, len(fields))
	ptrs := make([]*components.Autocomplete, len(fields))
	for i, f := range fields {
		a := components.NewAutocomplete(f.label, f.placeholder, m.th)
		a.SetValue(m.sim.values[f.key])
		a.SetSuggestions(f.suggestions)
		m.sim.fields[i] = a
		ptrs[i] = &m.sim.fields[i]
	}
	m.sim.form = components.NewForm(ptrs...)
	m.sim.panel = simPanelForm
	m.sim.form.Focus()
}

func (m *Model) handleSimFormKey(msg tea.KeyMsg) (screens.Screen, tea.Cmd) {
	f := &m.sim.form
	switch msg.String() {
	case "esc":
		m.sim.panel = simPanelRun
	case "shift+tab":
		f.Prev()
	case "enter":
		switch {
		case f.FocusedOpen():
			f.Update(msg)
		case !f.AtLast():
			f.Next()
		default:
			for i, fd := range simFields(simKinds[m.sim.kindIdx]) {
				m.sim.values[fd.key] = strings.TrimSpace(m.sim.fields[i].Value())
			}
			m.sim.panel = simPanelRun
			return m, m.runSimForm()
		}
	default:
		f.Update(msg)
	}
	return m, nil
}

// buildSimRequest turns the form values into a run request; money is entered in the base currency.
func buildSimRequest(kind string, v map[string]string, loc domain.Locale, cur domain.Currency) (simulating.RunRequest, error) {
	if kind == simulating.KindCashFlow {
		cf, err := buildCashFlow(v, loc, cur)
		return simulating.RunRequest{Kind: kind, CashFlow: &cf}, err
	}
	var p struct {
		initial, contribution        int64
		positions                    bool
		years                        int
		rates                        simulating.Rates
		cost, tax, inflation, growth int64
		compounding, frequency       string
	}
	var err error
	if p.initial, err = parseSimMoney("initial", v["initial"], loc, cur); err != nil {
		return simulating.RunRequest{}, err
	}
	switch strings.ToLower(strings.TrimSpace(v["positions"])) {
	case "yes", "y", "true":
		p.positions = true
	case "no", "n", "false", "":
	default:
		return simulating.RunRequest{}, simInvalid("positions", v["positions"])
	}
	if p.years, err = parseCount("years", v["years"]); err != nil {
		return simulating.RunRequest{}, err
	}
	if p.rates, err = parseSimRates(v["rates"]); err != nil {
		return simulating.RunRequest{}, err
	}
	for name, dst := range map[string]*int64{"cost": &p.cost, "tax": &p.tax, "inflation": &p.inflation, "growth": &p.growth} {
		if *dst, err = parsePercentBP(v[name]); err != nil {
			return simulating.RunRequest{}, simInvalid(name, v[name])
		}
	}
	p.compounding = strings.ToLower(strings.TrimSpace(v["compounding"]))
	if kind == simulating.KindInvestment {
		return simulating.RunRequest{Kind: kind, Investment: &simulating.InvestmentParams{
			InitialMinor: p.initial, InitialCurrentPositions: p.positions, Years: p.years,
			Compounding: p.compounding, Rates: p.rates, CostBP: p.cost, TaxBP: p.tax, InflationBP: p.inflation,
		}}, nil
	}
	if p.contribution, err = parseSimMoney("contribution", v["contribution"], loc, cur); err != nil {
		return simulating.RunRequest{}, err
	}
	return simulating.RunRequest{Kind: kind, PAC: &simulating.PACParams{
		InitialMinor: p.initial, InitialCurrentPositions: p.positions, ContributionMinor: p.contribution,
		Frequency: strings.ToLower(strings.TrimSpace(v["frequency"])), ContributionGrowthBP: p.growth,
		Years: p.years, Compounding: p.compounding, Rates: p.rates,
		CostBP: p.cost, TaxBP: p.tax, InflationBP: p.inflation,
	}}, nil
}

func simInvalid(field, value string) error {
	return fmt.Errorf("invalid %s %q", field, value)
}

func parseCount(field, s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, simInvalid(field, s)
	}
	return n, nil
}

func parseSimMoney(field, s string, loc domain.Locale, cur domain.Currency) (int64, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	money, err := domain.ParseAmount(s, loc, cur)
	if err != nil {
		return 0, simInvalid(field, s)
	}
	return money.Minor(), nil
}

// parseSimRates accepts one base rate (pessimistic and optimistic are 2 pp away) or three rates.
func parseSimRates(s string) (simulating.Rates, error) {
	if strings.TrimSpace(s) == "" {
		return simulating.Rates{}, simInvalid("rates", s)
	}
	parts := strings.Split(s, ",")
	bps := make([]int64, len(parts))
	for i, p := range parts {
		bp, err := parsePercentBP(p)
		if err != nil {
			return simulating.Rates{}, simInvalid("rates", s)
		}
		bps[i] = bp
	}
	switch len(bps) {
	case 1:
		return simulating.Rates{PessimisticBP: bps[0] - defaultRateSpreadBP, BaseBP: bps[0], OptimisticBP: bps[0] + defaultRateSpreadBP}, nil
	case 3:
		return simulating.Rates{PessimisticBP: bps[0], BaseBP: bps[1], OptimisticBP: bps[2]}, nil
	}
	return simulating.Rates{}, simInvalid("rates", s)
}

// parsePercentBP parses a percentage with up to two decimals into basis points; an empty value is 0.
func parsePercentBP(in string) (int64, error) {
	s := strings.ReplaceAll(strings.TrimSuffix(strings.TrimSpace(in), "%"), ",", ".")
	if s == "" {
		return 0, nil
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" && frac == "" || len(frac) > 2 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	if whole == "" {
		whole = "0"
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseUint(whole, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	f, err := strconv.ParseUint(frac, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	bp := int64(w)*100 + int64(f)
	if neg {
		bp = -bp
	}
	return bp, nil
}

// cashFlowEventFields maps each event form field to its event kind and the number of colon
// separated parts it takes; the syntax matches the `simulate cashflow` flags.
var cashFlowEventFields = []struct {
	key, kind string
	parts     int
}{
	{"oneoff", "one_off", 2},
	{"change", "recurring_change", 2},
	{"stopincome", "stop_income", 1},
	{"loan", "new_loan", 4},
}

func buildCashFlow(v map[string]string, loc domain.Locale, cur domain.Currency) (simulating.CashFlowParams, error) {
	var out simulating.CashFlowParams
	var err error
	if out.Months, err = parseCount("months", v["months"]); err != nil {
		return out, err
	}
	for _, f := range cashFlowEventFields {
		for _, entry := range strings.Split(v[f.key], ";") {
			if strings.TrimSpace(entry) == "" {
				continue
			}
			ev, err := parseSimEvent(f.key, f.kind, f.parts, entry, loc, cur)
			if err != nil {
				return out, err
			}
			out.Events = append(out.Events, ev)
		}
	}
	for _, a := range strings.Split(v["accounts"], ",") {
		if a = strings.TrimSpace(a); a != "" && !strings.EqualFold(a, "all") {
			out.Accounts = append(out.Accounts, a)
		}
	}
	return out, nil
}

func parseSimEvent(field, kind string, parts int, entry string, loc domain.Locale, cur domain.Currency) (simulating.CashFlowEvent, error) {
	fs := strings.Split(strings.TrimSpace(entry), ":")
	if len(fs) != parts {
		return simulating.CashFlowEvent{}, simInvalid(field, entry)
	}
	ev := simulating.CashFlowEvent{Kind: kind, Date: strings.TrimSpace(fs[0])}
	if parts > 1 {
		money, err := domain.ParseAmount(fs[1], loc, cur)
		if err != nil {
			return ev, simInvalid(field, entry)
		}
		ev.AmountMinor = money.Minor()
	}
	if parts == 4 {
		bp, err := parsePercentBP(fs[2])
		if err != nil {
			return ev, simInvalid(field, entry)
		}
		years, err := parseCount(field, fs[3])
		if err != nil {
			return ev, err
		}
		ev.RateBP, ev.Years = bp, years
	}
	return ev, nil
}
