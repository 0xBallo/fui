package config

import "time"

// Default values for standard FUI configuration.
const (
	DefaultCurrency      = "EUR"
	DefaultLocale        = LocaleIT
	DefaultMonthStartDay = 1
	DefaultWeekStart     = WeekStartMonday
	DefaultTheme         = ThemeTerminal
	DefaultBackupKeep    = 10
	DefaultTimeout       = 120 * time.Second
	DefaultLogLevel      = "warn"
)

// Defaults returns a new Config instance initialized with code-defined defaults.
func Defaults() Config {
	return Config{
		LogLevel:      DefaultLogLevel,
		Currency:      DefaultCurrency,
		Locale:        DefaultLocale,
		MonthStartDay: DefaultMonthStartDay,
		WeekStart:     DefaultWeekStart,
		Theme:         DefaultTheme,
		Backup: BackupConfig{
			Keep: DefaultBackupKeep,
		},
		Connectors: ConnectorsConfig{
			Timeout: Duration(DefaultTimeout),
		},
		Import: ImportConfig{
			Profiles: make(map[string]CSVProfile),
		},
	}
}
