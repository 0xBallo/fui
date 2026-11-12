// Package config provides typed configuration loading, validation, and XDG
// path resolution for FUI.
//
// Configuration precedence follows: command-line flags > FUI_* environment
// variables > config.toml file > code defaults. Dotted key access is supported
// for reading and updating settings on disk.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/module"
)

// Supported locales.
const (
	LocaleIT = "it-IT"
	LocaleUS = "en-US"
)

// Supported themes.
const (
	ThemeAuto         = "auto"
	ThemeLatte        = "latte"
	ThemeFrappe       = "frappe"
	ThemeMacchiato    = "macchiato"
	ThemeMocha        = "mocha"
	ThemeTerminal     = "terminal"
	ThemeHighContrast = "high-contrast"
)

// themeFlavors lists the fixed Catppuccin flavors theme_light and theme_dark may select.
var themeFlavors = []string{ThemeLatte, ThemeFrappe, ThemeMacchiato, ThemeMocha}

// Supported week starts.
const (
	WeekStartMonday = "monday"
	WeekStartSunday = "sunday"
)

// Supported budget modes.
const (
	BudgetModeCap      = "cap"
	BudgetModeEnvelope = "envelope"
)

// Duration wraps time.Duration to provide TOML text marshaling and unmarshaling.
type Duration time.Duration

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String returns the formatted duration string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// UnmarshalText decodes duration strings (e.g., "120s", "2m") or integer seconds.
func (d *Duration) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" {
		*d = Duration(0)
		return nil
	}

	parsed, err := time.ParseDuration(s)
	if err == nil {
		*d = Duration(parsed)
		return nil
	}

	sec, parseErr := strconv.Atoi(s)
	if parseErr == nil {
		*d = Duration(time.Duration(sec) * time.Second)
		return nil
	}

	return fmt.Errorf("invalid duration %q: %w", s, err)
}

// MarshalText encodes duration as a standard string representation.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// CSVProfile defines column mapping and formatting rules for CSV bank statement imports.
type CSVProfile struct {
	DateColumn        string `toml:"date_column"`
	AmountColumn      string `toml:"amount_column,omitempty"`
	DebitColumn       string `toml:"debit_column,omitempty"`
	CreditColumn      string `toml:"credit_column,omitempty"`
	DescriptionColumn string `toml:"description_column"`
	ExternalIDColumn  string `toml:"external_id_column,omitempty"`
	Delimiter         string `toml:"delimiter"`
	Encoding          string `toml:"encoding,omitempty"`
	DateFormat        string `toml:"date_format"`
	DecimalSeparator  string `toml:"decimal_separator"`
	SkipRows          int    `toml:"skip_rows,omitempty"`
	TargetAccount     string `toml:"target_account,omitempty"`
}

// BackupConfig holds backup rotation settings.
type BackupConfig struct {
	Keep int `toml:"keep"`
}

// ConnectorsConfig holds execution settings for connectors.
type ConnectorsConfig struct {
	Timeout Duration `toml:"timeout"`
}

// ImportConfig holds ingestion profiles.
type ImportConfig struct {
	Profiles map[string]CSVProfile `toml:"profiles"`
}

// ForecastConfig holds forecast settings.
type ForecastConfig struct {
	Horizon int `toml:"horizon"`
}

// ModulesConfig holds enabled module ids.
type ModulesConfig struct {
	Enabled []module.ID `toml:"enabled"`
}

// BudgetConfig holds the budget module's settings.
type BudgetConfig struct {
	// Mode selects how budgets are evaluated: "cap" (spending caps) or "envelope".
	Mode string `toml:"mode"`
}

// Config represents the complete configuration and runtime environment of FUI.
type Config struct {
	// Resolved paths and operational options (not written to TOML).
	ConfigDir    string `toml:"-"`
	ConfigFile   string `toml:"-"`
	DataDir      string `toml:"-"`
	DBPath       string `toml:"-"`
	StateDir     string `toml:"-"`
	BackupDir    string `toml:"-"`
	LogPath      string `toml:"-"`
	LogLevel     string `toml:"-"`
	LogSensitive bool   `toml:"-"`
	AgentMode    bool   `toml:"-"`

	// Persistent configuration fields stored in config.toml.
	Currency      string           `toml:"currency"`
	Locale        string           `toml:"locale"`
	MonthStartDay int              `toml:"month_start_day"`
	WeekStart     string           `toml:"week_start"`
	Theme         string           `toml:"theme"`
	ThemeLight    string           `toml:"theme_light"`
	ThemeDark     string           `toml:"theme_dark"`
	Modules       ModulesConfig    `toml:"modules"`
	Budget        BudgetConfig     `toml:"budget"`
	Backup        BackupConfig     `toml:"backup"`
	Connectors    ConnectorsConfig `toml:"connectors"`
	Forecast      ForecastConfig   `toml:"forecast"`
	Import        ImportConfig     `toml:"import"`
}
