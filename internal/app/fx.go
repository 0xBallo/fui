package app

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/0xballo/fui/internal/domain"
)

// maxRateIntDigits keeps the scaled rate below the int64 range.
const maxRateIntDigits = 9

// ParseRate parses an exchange rate typed as decimal text, e.g. "1.0842" or "0,92": '.' or ','
// is the decimal separator, there is no thousands separator, and at most 9 decimals are kept
// exactly (more are rejected rather than rounded).
func ParseRate(s string) (domain.Rate, error) {
	in := strings.TrimSpace(s)
	whole, frac, _ := strings.Cut(strings.Replace(in, ",", ".", 1), ".")
	digits := whole + frac
	bad := digits == "" || len(whole) > maxRateIntDigits || len(frac) > 9 || strings.ContainsAny(frac, ".,")
	for _, c := range digits {
		bad = bad || c < '0' || c > '9'
	}
	if bad {
		return 0, newError(domain.ErrInvalidValue, map[string]any{"field": "rate", "value": s}, "invalid rate %q", s)
	}
	var scaled int64
	for _, c := range whole + frac + strings.Repeat("0", 9-len(frac)) {
		scaled = scaled*10 + int64(c-'0')
	}
	r, err := domain.NewRate(scaled)
	if err != nil {
		return 0, fmt.Errorf("rate: %w", err)
	}
	return r, nil
}

// SetFXRateRequest sets the rate of Currency on Date (empty = today): one unit of Currency is
// worth Rate units of the base currency.
type SetFXRateRequest struct {
	Currency string
	Date     string
	Rate     string
}

// SetFXRatePlan is the plan to store Rate.
type SetFXRatePlan struct {
	Plan
	Rate domain.FXRate
}

// PlanSetFXRate validates the request; it plans an update when the rate of that day already exists.
func (s *Service) PlanSetFXRate(ctx context.Context, req SetFXRateRequest) (SetFXRatePlan, error) {
	if err := s.needBase(); err != nil {
		return SetFXRatePlan{}, err
	}
	cur, err := domain.ParseCurrency(req.Currency)
	if err != nil {
		return SetFXRatePlan{}, fmt.Errorf("currency: %w", err)
	}
	if cur == s.cfg.BaseCurrency {
		return SetFXRatePlan{}, newError(domain.ErrInvalidValue, map[string]any{"field": "currency", "value": cur.Code()},
			"%s is the base currency: its rate is always 1", cur)
	}
	date, err := s.date(req.Date)
	if err != nil {
		return SetFXRatePlan{}, err
	}
	rate, err := ParseRate(req.Rate)
	if err != nil {
		return SetFXRatePlan{}, err
	}
	fx, err := domain.NewFXRate(cur, date, rate)
	if err != nil {
		return SetFXRatePlan{}, fmt.Errorf("fx rate: %w", err)
	}
	op := OpCreate
	err = s.store.Read(ctx, func(r Repos) error {
		prev, err := r.FXRates().OnOrBefore(ctx, cur, date)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return nil
		case err != nil:
			return fmt.Errorf("look up fx rate: %w", err)
		}
		if prev.Date == date {
			op = OpUpdate
		}
		return nil
	})
	if err != nil {
		return SetFXRatePlan{}, err
	}
	ch := Change{
		Op: op, Entity: EntityFXRate,
		Summary: fmt.Sprintf("%s fx rate 1 %s = %s %s on %s", op, cur, formatRate(rate), s.cfg.BaseCurrency, date),
		Fields: map[string]any{
			"currency": cur.Code(), "date": date.String(), "rate": rate.Scaled(), "base_currency": s.cfg.BaseCurrency.Code(),
		},
	}
	return SetFXRatePlan{Plan: Plan{Changes: []Change{ch}}, Rate: fx}, nil
}

// ApplySetFXRate stores the rate, replacing any rate of the same currency and date.
func (s *Service) ApplySetFXRate(ctx context.Context, p SetFXRatePlan) (domain.FXRate, error) {
	err := s.apply(ctx, p.Bulk, func(r Repos) error {
		if err := r.FXRates().Upsert(ctx, p.Rate); err != nil {
			return fmt.Errorf("store fx rate: %w", err)
		}
		return nil
	})
	return p.Rate, err
}

// ListFXRates returns the stored rates of currency (every currency when empty).
func (s *Service) ListFXRates(ctx context.Context, currency string) ([]domain.FXRate, error) {
	var cur domain.Currency
	if currency != "" {
		var err error
		if cur, err = domain.ParseCurrency(currency); err != nil {
			return nil, fmt.Errorf("currency: %w", err)
		}
	}
	var out []domain.FXRate
	err := s.store.Read(ctx, func(r Repos) (err error) {
		out, err = r.FXRates().List(ctx, cur)
		return err
	})
	return out, err
}

// LookupFXRate returns the rate in force for currency on date (empty = today): the latest one
// dated on or before it. The base currency is always 1. A missing rate is ErrRateMissing.
func (s *Service) LookupFXRate(ctx context.Context, currency, date string) (domain.FXRate, error) {
	cur, err := domain.ParseCurrency(currency)
	if err != nil {
		return domain.FXRate{}, fmt.Errorf("currency: %w", err)
	}
	d, err := s.date(date)
	if err != nil {
		return domain.FXRate{}, err
	}
	var out domain.FXRate
	err = s.store.Read(ctx, func(r Repos) error {
		rate, err := s.rateOn(ctx, r, cur, d)
		out = domain.FXRate{Currency: cur, Date: d, Rate: rate}
		return err
	})
	return out, err
}

func (s *Service) needBase() error {
	if s.cfg.BaseCurrency.IsZero() {
		return newError(domain.ErrMissingField, map[string]any{"field": "base_currency"}, "no base currency configured")
	}
	return nil
}

// rateOn returns the rate of cur valid on date; the base currency is worth exactly 1.
func (s *Service) rateOn(ctx context.Context, r Repos, cur domain.Currency, date domain.Date) (domain.Rate, error) {
	if err := s.needBase(); err != nil {
		return 0, err
	}
	if cur == s.cfg.BaseCurrency {
		return domain.Rate(domain.RateScale), nil
	}
	fx, err := r.FXRates().OnOrBefore(ctx, cur, date)
	if errors.Is(err, domain.ErrNotFound) {
		return 0, missingRates([]domain.Currency{cur}, date)
	}
	if err != nil {
		return 0, fmt.Errorf("look up %s rate: %w", cur, err)
	}
	return fx.Rate, nil
}

func missingRates(cs []domain.Currency, date domain.Date) error {
	codes := make([]string, len(cs))
	for i, c := range cs {
		codes[i] = c.Code()
	}
	slices.Sort(codes)
	return newError(ErrRateMissing, map[string]any{"currencies": codes, "date": date.String()},
		"no exchange rate on or before %s for %s", date, strings.Join(codes, ", "))
}

// convertVia converts m into currency to through the base currency, rounding once:
// to = m × from / toRate, where from and toRate are the base value of one unit of each currency.
func convertVia(m domain.Money, to domain.Currency, from, toRate domain.Rate) (domain.Money, error) {
	num := new(big.Int).Mul(big.NewInt(m.Minor()), big.NewInt(from.Scaled()))
	num.Mul(num, pow10(to.Decimals()))
	den := new(big.Int).Mul(big.NewInt(toRate.Scaled()), pow10(m.Currency().Decimals()))
	minor, err := domain.RoundDiv(num, den)
	if err != nil {
		return domain.Money{}, fmt.Errorf("convert %s to %s: %w", m.Currency(), to, err)
	}
	out, err := domain.NewMoney(minor, to)
	if err != nil {
		return domain.Money{}, fmt.Errorf("convert %s to %s: %w", m.Currency(), to, err)
	}
	return out, nil
}

func pow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

// formatRate renders a scaled rate with its significant decimals, e.g. 1084200000 → "1.0842".
func formatRate(r domain.Rate) string {
	whole, frac := r.Scaled()/domain.RateScale, r.Scaled()%domain.RateScale
	if frac == 0 {
		return fmt.Sprint(whole)
	}
	return fmt.Sprintf("%d.%s", whole, strings.TrimRight(fmt.Sprintf("%09d", frac), "0"))
}

// NetWorthRequest selects the date (empty = today) and whether archived accounts are counted.
type NetWorthRequest struct {
	At              string
	IncludeArchived bool
}

// NetWorthLine is one account balance and its value in the base currency.
type NetWorthLine struct {
	Account   domain.Account
	Balance   domain.Money
	Converted domain.Money
}

// NetWorth is the sum of the account balances converted to the base currency.
type NetWorth struct {
	At    domain.Date
	Base  domain.Currency
	Total domain.Money
	Lines []NetWorthLine
}

// NetWorth sums the balances at the requested date, each converted at the rate in force on that
// date. If rates are missing it fails with ErrRateMissing listing every currency lacking one.
func (s *Service) NetWorth(ctx context.Context, req NetWorthRequest) (NetWorth, error) {
	if err := s.needBase(); err != nil {
		return NetWorth{}, err
	}
	at, err := s.date(req.At)
	if err != nil {
		return NetWorth{}, err
	}
	out := NetWorth{At: at, Base: s.cfg.BaseCurrency}
	out.Total, _ = domain.NewMoney(0, out.Base)
	err = s.store.Read(ctx, func(r Repos) error {
		accts, err := r.Accounts().List(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		if !req.IncludeArchived {
			accts = activeAccounts(accts)
		}
		var missing []domain.Currency
		for _, a := range accts {
			line, err := s.netWorthLine(ctx, r, a, at)
			if errors.Is(err, ErrRateMissing) {
				if !slices.Contains(missing, a.Currency) {
					missing = append(missing, a.Currency)
				}
				continue
			}
			if err != nil {
				return err
			}
			if out.Total, err = out.Total.Add(line.Converted); err != nil {
				return fmt.Errorf("net worth: %w", err)
			}
			out.Lines = append(out.Lines, line)
		}
		if len(missing) > 0 {
			return missingRates(missing, at)
		}
		return nil
	})
	return out, err
}

func (s *Service) netWorthLine(ctx context.Context, r Repos, a domain.Account, at domain.Date) (NetWorthLine, error) {
	bal, err := balanceAt(ctx, r, a, at)
	if err != nil {
		return NetWorthLine{}, err
	}
	rate, err := s.rateOn(ctx, r, a.Currency, at)
	if err != nil {
		return NetWorthLine{}, err
	}
	conv, err := domain.Convert(bal, rate, s.cfg.BaseCurrency)
	if err != nil {
		return NetWorthLine{}, fmt.Errorf("convert %q: %w", a.Name, err)
	}
	return NetWorthLine{Account: a, Balance: bal, Converted: conv}, nil
}
