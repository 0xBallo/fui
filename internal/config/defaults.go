package config

import (
	"time"

	"github.com/0xballo/fui/internal/module"
)

// Default values for standard FUI configuration.
const (
	DefaultCurrency      = "EUR"
	DefaultLocale        = LocaleIT
	DefaultMonthStartDay = 1
	DefaultWeekStart     = WeekStartMonday
	DefaultTheme         = ThemeAuto
	DefaultThemeLight    = ThemeLatte
	DefaultThemeDark     = ThemeMocha
	DefaultBackupKeep    = 10
	DefaultTimeout       = 120 * time.Second
	DefaultForecastDays  = 90
	DefaultLogLevel      = "warn"
	DefaultBudgetMode    = BudgetModeCap
)

// Defaults returns a new Config instance initialized with code-defined defaults.
func Defaults() Config {
	defaultModules := module.Default()
	return Config{
		LogLevel:      DefaultLogLevel,
		Currency:      DefaultCurrency,
		Locale:        DefaultLocale,
		MonthStartDay: DefaultMonthStartDay,
		WeekStart:     DefaultWeekStart,
		Theme:         DefaultTheme,
		ThemeLight:    DefaultThemeLight,
		ThemeDark:     DefaultThemeDark,
		Modules: ModulesConfig{
			Enabled: defaultModules.IDs(),
		},
		Budget: BudgetConfig{
			Mode: DefaultBudgetMode,
		},
		Backup: BackupConfig{
			Keep: DefaultBackupKeep,
		},
		Connectors: ConnectorsConfig{
			Timeout: Duration(DefaultTimeout),
		},
		Forecast: ForecastConfig{
			Horizon: DefaultForecastDays,
		},
		Insights: defaultInsights(),
		Import: ImportConfig{
			Profiles: make(map[string]CSVProfile),
		},
	}
}
