package reporting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// DefaultHorizonDays is the forecast horizon when Config.HorizonDays is unset.
const DefaultHorizonDays = 90

// Config holds the settings the analytics use cases depend on.
type Config struct {
	Locale        domain.Locale   // amount input format; default en-US
	BaseCurrency  domain.Currency // totals are converted into it
	MonthStartDay int             // 1..31; default 1
	BudgetMode    domain.BudgetMode
	HorizonDays   int            // forecast.horizon in days; default 90
	Location      *time.Location // zone that defines "today"; default time.Local
	Modules       module.Set     // enabled modules; analytics must be among them
}

// Analytics runs the analytics module use cases (forecast, reports, saved reports) over a Store.
type Analytics struct {
	store app.Store
	clock app.Clock
	cfg   Config
}

// NewAnalytics builds an Analytics, defaulting Locale, MonthStartDay, BudgetMode, HorizonDays
// and Location when unset.
func NewAnalytics(store app.Store, clock app.Clock, cfg Config) *Analytics {
	if cfg.Locale == "" {
		cfg.Locale = domain.LocaleENUS
	}
	if cfg.MonthStartDay == 0 {
		cfg.MonthStartDay = 1
	}
	if cfg.BudgetMode == "" {
		cfg.BudgetMode = domain.BudgetCap
	}
	if cfg.HorizonDays == 0 {
		cfg.HorizonDays = DefaultHorizonDays
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return &Analytics{store: store, clock: clock, cfg: cfg}
}

// gate fails with module_disabled unless analytics and every extra module are enabled.
func (a *Analytics) gate(extra ...module.ID) error {
	for _, id := range append([]module.ID{module.Analytics}, extra...) {
		if err := a.cfg.Modules.Require(id); err != nil {
			return app.WrapModuleError(err)
		}
	}
	return nil
}

func (a *Analytics) today() (domain.Date, error) {
	d, err := domain.FromTime(a.clock.Now().In(a.cfg.Location))
	if err != nil {
		return domain.Date{}, fmt.Errorf("today: %w", err)
	}
	return d, nil
}

// Amount is a money value in JSON-friendly form.
type Amount struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
}

func amountOf(m domain.Money) Amount { return Amount{Minor: m.Minor(), Currency: m.Currency().Code()} }

// Point is a dated amount of a series.
type Point struct {
	Date   string `json:"date"`
	Amount Amount `json:"amount"`
}

func zeroMoney(c domain.Currency) domain.Money {
	m, _ := domain.NewMoney(0, c) // a valid currency never fails
	return m
}

// toBase converts m to the base currency at the rate on or before date.
func (a *Analytics) toBase(ctx context.Context, r app.Repos, m domain.Money, date domain.Date) (domain.Money, error) {
	if m.Currency() == a.cfg.BaseCurrency {
		return m, nil
	}
	fx, err := r.FXRates().OnOrBefore(ctx, m.Currency(), date)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Money{}, &domain.Error{
			Code:    app.CodeRateMissing,
			Message: fmt.Sprintf("no exchange rate on or before %s for %s", date, m.Currency()),
			Details: map[string]any{"currencies": []string{m.Currency().Code()}, "date": date.String()},
		}
	}
	if err != nil {
		return domain.Money{}, fmt.Errorf("look up %s rate: %w", m.Currency(), err)
	}
	return domain.Convert(m, fx.Rate, a.cfg.BaseCurrency)
}

// balanceAt is the balance of acct at date: zero before the account was opened.
func balanceAt(ctx context.Context, r app.Repos, acct domain.Account, date domain.Date) (domain.Money, error) {
	if date.Before(acct.OpenedOn) {
		return zeroMoney(acct.Currency), nil
	}
	sum, err := r.Transactions().SumUpTo(ctx, acct.ID, date)
	if err != nil {
		return domain.Money{}, fmt.Errorf("balance of %q: %w", acct.Name, err)
	}
	mv, err := domain.NewMoney(sum, acct.Currency)
	if err != nil {
		return domain.Money{}, err
	}
	return acct.OpeningBalance.Add(mv)
}

// monthEnds returns the last day of every accounting month touching p, the last one clipped to p.
func (a *Analytics) monthEnds(p domain.Period) ([]domain.Date, error) {
	cur, err := domain.MonthPeriod(p.From(), a.cfg.MonthStartDay)
	if err != nil {
		return nil, err
	}
	var out []domain.Date
	for {
		end := cur.To()
		if end.After(p.To()) {
			end = p.To()
		}
		out = append(out, end)
		if !cur.To().Before(p.To()) {
			return out, nil
		}
		next, err := cur.To().AddDays(1)
		if err != nil {
			return nil, err
		}
		if cur, err = domain.MonthPeriod(next, a.cfg.MonthStartDay); err != nil {
			return nil, err
		}
	}
}

func pickAccount(accts []domain.Account, ref string) (domain.Account, error) {
	names := make([]string, len(accts))
	for i, ac := range accts {
		names[i] = ac.Name
	}
	i, err := domain.Resolve(ref, names)
	if err != nil {
		return domain.Account{}, fmt.Errorf("account: %w", err)
	}
	return accts[i], nil
}
