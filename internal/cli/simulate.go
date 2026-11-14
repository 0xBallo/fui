package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/cli/render"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// SimulateModule is the module that owns the simulate command.
const SimulateModule = module.Simulations

// defaultRateSpreadBP is the distance of the pessimistic and optimistic rates from a lone --rate.
const defaultRateSpreadBP = 200

// NewSimulateCmd builds `fui simulate`; the root command registers it under the simulations
// module. svc is built by the composition root over the same store and clock as the rest of the
// deps; nil makes every subcommand fail cleanly instead of panicking.
func NewSimulateCmd(deps *Deps, svc *simulating.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "Run, save and compare hypothetical scenarios without touching your data",
	}
	cmd.AddCommand(
		newSimulateRunCmd(deps, svc, simulating.KindInvestment),
		newSimulateRunCmd(deps, svc, simulating.KindPAC),
		newSimulateRunCmd(deps, svc, simulating.KindCashFlow),
		newSimulateSaveCmd(deps, svc),
		newSimulateListCmd(deps, svc),
		newSimulateCompareCmd(deps, svc),
		newSimulateRmCmd(deps, svc),
	)
	return cmd
}

func requireSimulating(svc *simulating.Service) error {
	if svc == nil {
		return fmt.Errorf("fui: simulations service not configured")
	}
	return nil
}

var simulateExamples = map[string]string{
	simulating.KindInvestment: "  fui simulate investment --initial 10000 --years 10 --rate 5\n" +
		"  fui simulate investment --from-positions --years 15 --rate 3 --rate 5 --rate 7 --chart",
	simulating.KindPAC: "  fui simulate pac --monthly 200 --years 20 --rate 4,5\n" +
		"  fui simulate pac --quarterly 600 --initial 1000 --years 10 --rate 5 --tax 26 --inflation 2",
	simulating.KindCashFlow: "  fui simulate cashflow --months 12 --one-off 2027-03-01:-5000\n" +
		"  fui simulate cashflow --stop-income 2027-01-01 --loan 2027-02-01:20000:4,5:5",
}

var simulateShorts = map[string]string{
	simulating.KindInvestment: "Simulate an investment at three annual rates",
	simulating.KindPAC:        "Simulate a periodic accumulation plan at three annual rates",
	simulating.KindCashFlow:   "Simulate hypothetical events on top of the cash forecast",
}

func newSimulateRunCmd(deps *Deps, svc *simulating.Service, kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     kind,
		Short:   simulateShorts[kind],
		Example: simulateExamples[kind],
		Args:    noArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			if err := requireSimulating(svc); err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			req, err := simulateRequest(cmd, deps, kind)
			if err != nil {
				return err
			}
			out, err := svc.Run(cmd.Context(), enabled, req)
			if err != nil {
				return err
			}
			cur, err := deps.BaseCurrency()
			if err != nil {
				return err
			}
			chart, _ := cmd.Flags().GetBool("chart")
			return render.WriteSimulationRun(deps.Stdout, rt.format, deps.Locale(), rt.color, cur, req, out, chart)
		},
	}
	registerSimulateFlags(cmd, kind)
	reportChartFlag(cmd)
	return cmd
}

func newSimulateSaveCmd(deps *Deps, svc *simulating.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save NAME KIND",
		Short: "Save a simulation scenario (KIND: investment, pac or cashflow)",
		Example: "  fui simulate save retirement pac --monthly 200 --years 20 --rate 4,5\n" +
			"  fui simulate save sabbatical cashflow --stop-income 2027-01-01",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			if err := requireSimulating(svc); err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			kind := strings.ToLower(strings.TrimSpace(args[1]))
			if _, ok := simulateShorts[kind]; !ok {
				return newUsageError(fmt.Errorf("unknown simulation kind %q: want investment, pac or cashflow", args[1]))
			}
			req, err := simulateRequest(cmd, deps, kind)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanSave(ctx, enabled, args[0], req)
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				saved, err := svc.ApplySave(ctx, plan)
				if err != nil {
					return err
				}
				return render.WriteSimulationSaved(deps.Stdout, rt.format, saved)
			})
		},
	}
	for _, kind := range []string{simulating.KindInvestment, simulating.KindPAC, simulating.KindCashFlow} {
		registerSimulateFlags(cmd, kind)
	}
	return cmd
}

func newSimulateListCmd(deps *Deps, svc *simulating.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List saved simulations",
		Example: "  fui simulate list --json",
		Args:    noArgs(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			if err := requireSimulating(svc); err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			sims, err := svc.List(cmd.Context(), enabled)
			if err != nil {
				return err
			}
			return render.WriteSimulationList(deps.Stdout, rt.format, rt.color, sims)
		},
	}
}

func newSimulateCompareCmd(deps *Deps, svc *simulating.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "compare NAME NAME [NAME]",
		Short:   "Rerun 2-3 saved simulations and show them side by side",
		Example: "  fui simulate compare retirement sabbatical",
		Args: func(cmd *cobra.Command, args []string) error {
			return newUsageError(cobra.RangeArgs(2, 3)(cmd, args))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			if err := requireSimulating(svc); err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			out, err := svc.Compare(cmd.Context(), enabled, args)
			if err != nil {
				return err
			}
			cur, err := deps.BaseCurrency()
			if err != nil {
				return err
			}
			return render.WriteSimulationCompare(deps.Stdout, rt.format, deps.Locale(), rt.color, cur, out)
		},
	}
}

func newSimulateRmCmd(deps *Deps, svc *simulating.Service) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Remove a saved simulation",
		Example: "  fui simulate rm sabbatical",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			if err := requireSimulating(svc); err != nil {
				return err
			}
			enabled, err := enabledModulesFromDeps(deps)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			plan, err := svc.PlanRemove(ctx, enabled, args[0])
			if err != nil {
				return err
			}
			return writeChange(cmd, deps, rt, plan.Plan, func() error {
				if err := svc.ApplyRemove(ctx, plan); err != nil {
					return err
				}
				name, _ := plan.Changes[0].Fields["name"].(string)
				return render.WriteSimulationRemoved(deps.Stdout, rt.format, name)
			})
		},
	}
}

func registerSimulateFlags(cmd *cobra.Command, kind string) {
	f := cmd.Flags()
	if kind == simulating.KindCashFlow {
		f.Int("months", 12, "horizon in months")
		f.StringArray("one-off", nil, "one-off flow `DATE:AMOUNT` (negative for an expense); repeatable")
		f.StringArray("change", nil, "monthly change from a date `DATE:AMOUNT` (e.g. a raise); repeatable")
		f.StringArray("stop-income", nil, "stop all income from `DATE`; repeatable")
		f.StringArray("loan", nil, "new loan `DATE:AMOUNT:RATE:YEARS`; repeatable")
		return
	}
	if f.Lookup("years") == nil {
		f.Int("years", 0, "horizon in years")
		f.String("initial", "", "initial capital")
		f.Bool("from-positions", false, "start from the current value of your investment positions")
		f.StringArray("rate", nil, "annual return in percent: one value (base, ±spread) or three (low, base, high)")
		f.String("rate-spread", "", "percent distance of low/high from a single --rate (default 2)")
		f.String("compounding", "annual", "compounding: monthly or annual")
		f.String("cost", "", "annual costs in percent")
		f.String("tax", "", "tax rate on gains in percent")
		f.String("inflation", "", "annual inflation in percent")
	}
	if kind == simulating.KindPAC && f.Lookup("monthly") == nil {
		f.String("monthly", "", "monthly contribution")
		f.String("quarterly", "", "quarterly contribution")
		f.String("yearly", "", "yearly contribution")
		f.String("growth", "", "yearly growth of the contribution in percent")
	}
}

func simulateRequest(cmd *cobra.Command, deps *Deps, kind string) (simulating.RunRequest, error) {
	cur, err := deps.BaseCurrency()
	if err != nil {
		return simulating.RunRequest{}, err
	}
	p := simFlagParser{cmd: cmd, loc: deps.Locale(), cur: cur}
	req := simulating.RunRequest{Kind: kind}
	if kind == simulating.KindCashFlow {
		cf, err := p.cashFlow()
		req.CashFlow = &cf
		return req, err
	}
	common, err := p.investment()
	if err != nil {
		return req, err
	}
	if kind == simulating.KindInvestment {
		req.Investment = &common
		return req, nil
	}
	pac, err := p.pac(common)
	req.PAC = &pac
	return req, err
}

type simFlagParser struct {
	cmd *cobra.Command
	loc domain.Locale
	cur domain.Currency
}

func (p simFlagParser) investment() (simulating.InvestmentParams, error) {
	f := p.cmd.Flags()
	var out simulating.InvestmentParams
	var err error
	out.Years, _ = f.GetInt("years")
	out.InitialCurrentPositions, _ = f.GetBool("from-positions")
	out.Compounding, _ = f.GetString("compounding")
	if out.InitialMinor, err = p.amount("initial"); err != nil {
		return out, err
	}
	if out.Rates, err = p.rates(); err != nil {
		return out, err
	}
	if out.CostBP, err = p.percent("cost"); err != nil {
		return out, err
	}
	if out.TaxBP, err = p.percent("tax"); err != nil {
		return out, err
	}
	out.InflationBP, err = p.percent("inflation")
	return out, err
}

func (p simFlagParser) pac(in simulating.InvestmentParams) (simulating.PACParams, error) {
	out := simulating.PACParams{
		InitialMinor: in.InitialMinor, InitialCurrentPositions: in.InitialCurrentPositions,
		Years: in.Years, Compounding: in.Compounding, Rates: in.Rates,
		CostBP: in.CostBP, TaxBP: in.TaxBP, InflationBP: in.InflationBP,
	}
	var err error
	set := 0
	for _, freq := range []string{"monthly", "quarterly", "yearly"} {
		if !p.cmd.Flags().Changed(freq) {
			continue
		}
		set++
		out.Frequency = freq
		if out.ContributionMinor, err = p.amount(freq); err != nil {
			return out, err
		}
	}
	if set != 1 {
		return out, newUsageError(fmt.Errorf("exactly one of --monthly, --quarterly, --yearly is required"))
	}
	out.ContributionGrowthBP, err = p.percent("growth")
	return out, err
}

func (p simFlagParser) cashFlow() (simulating.CashFlowParams, error) {
	var out simulating.CashFlowParams
	out.Months, _ = p.cmd.Flags().GetInt("months")
	specs := []struct {
		flag, kind string
		fields     int
	}{
		{"one-off", "one_off", 2},
		{"change", "recurring_change", 2},
		{"stop-income", "stop_income", 1},
		{"loan", "new_loan", 4},
	}
	for _, spec := range specs {
		values, _ := p.cmd.Flags().GetStringArray(spec.flag)
		for _, v := range values {
			ev, err := p.event(spec.flag, spec.kind, spec.fields, v)
			if err != nil {
				return out, err
			}
			out.Events = append(out.Events, ev)
		}
	}
	return out, nil
}

func (p simFlagParser) event(flag, kind string, fields int, value string) (simulating.CashFlowEvent, error) {
	parts := strings.Split(value, ":")
	if len(parts) != fields {
		return simulating.CashFlowEvent{}, invalidFlag(flag, value)
	}
	ev := simulating.CashFlowEvent{Kind: kind, Date: strings.TrimSpace(parts[0])}
	if fields > 1 {
		m, err := domain.ParseAmount(parts[1], p.loc, p.cur)
		if err != nil {
			return ev, fmt.Errorf("--%s %q: %w", flag, value, err)
		}
		ev.AmountMinor = m.Minor()
	}
	if fields == 4 {
		bp, err := parsePercentBP(parts[2])
		if err != nil {
			return ev, invalidFlag(flag, value)
		}
		years, err := strconv.Atoi(strings.TrimSpace(parts[3]))
		if err != nil {
			return ev, invalidFlag(flag, value)
		}
		ev.RateBP, ev.Years = bp, years
	}
	return ev, nil
}

func (p simFlagParser) amount(flag string) (int64, error) {
	in, _ := p.cmd.Flags().GetString(flag)
	if strings.TrimSpace(in) == "" {
		return 0, nil
	}
	m, err := domain.ParseAmount(in, p.loc, p.cur)
	if err != nil {
		return 0, fmt.Errorf("--%s %q: %w", flag, in, err)
	}
	return m.Minor(), nil
}

func (p simFlagParser) percent(flag string) (int64, error) {
	in, _ := p.cmd.Flags().GetString(flag)
	if strings.TrimSpace(in) == "" {
		return 0, nil
	}
	bp, err := parsePercentBP(in)
	if err != nil {
		return 0, invalidFlag(flag, in)
	}
	return bp, nil
}

func (p simFlagParser) rates() (simulating.Rates, error) {
	raw, _ := p.cmd.Flags().GetStringArray("rate")
	bps := make([]int64, len(raw))
	for i, r := range raw {
		bp, err := parsePercentBP(r)
		if err != nil {
			return simulating.Rates{}, invalidFlag("rate", r)
		}
		bps[i] = bp
	}
	switch len(bps) {
	case 3:
		return simulating.Rates{PessimisticBP: bps[0], BaseBP: bps[1], OptimisticBP: bps[2]}, nil
	case 1:
		spread := int64(defaultRateSpreadBP)
		if s, _ := p.cmd.Flags().GetString("rate-spread"); strings.TrimSpace(s) != "" {
			var err error
			if spread, err = parsePercentBP(s); err != nil {
				return simulating.Rates{}, invalidFlag("rate-spread", s)
			}
		}
		return simulating.Rates{PessimisticBP: bps[0] - spread, BaseBP: bps[0], OptimisticBP: bps[0] + spread}, nil
	case 0:
		return simulating.Rates{}, newUsageError(fmt.Errorf("--rate is required (one base rate or three: low, base, high)"))
	}
	return simulating.Rates{}, newUsageError(fmt.Errorf("--rate takes one or three values, got %d", len(bps)))
}

func invalidFlag(flag, value string) error {
	return &domain.Error{
		Code:    domain.CodeInvalidValue,
		Message: fmt.Sprintf("invalid --%s %q", flag, value),
		Details: map[string]any{"field": flag, "value": value},
	}
}

// parsePercentBP converts a percentage such as "4,5", "4.5" or "-1.25%" to basis points. Both
// decimal separators are accepted whatever the locale, since a percentage has no grouping.
func parsePercentBP(in string) (int64, error) {
	s := strings.TrimSuffix(strings.TrimSpace(in), "%")
	s = strings.ReplaceAll(s, ",", ".")
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" && frac == "" || len(frac) > 2 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseInt(orZero(whole), 10, 64)
	if err != nil || w < 0 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("invalid percentage %q", in)
	}
	bp := w*100 + f
	if neg {
		bp = -bp
	}
	return bp, nil
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
