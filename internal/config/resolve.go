package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// List of supported environment variables.
// - FUI_CONFIG: Custom configuration file path.
// - FUI_DB: Custom database file path.
// - FUI_CURRENCY: Base currency override.
// - FUI_LOCALE: Regional formatting locale override.
// - FUI_MONTH_START_DAY: Accounting month start day override (1..31).
// - FUI_WEEK_START: First day of week override (monday/sunday).
// - FUI_THEME: TUI theme override (auto/latte/frappe/macchiato/mocha/terminal/high-contrast).
// - FUI_THEME_LIGHT: Flavor auto picks on a light terminal background.
// - FUI_THEME_DARK: Flavor auto picks on a dark terminal background.
// - FUI_BACKUP_KEEP: Number of backups retained during rotation.
// - FUI_CONNECTORS_TIMEOUT: Connector execution timeout duration.
// - FUI_LOG: Logging severity level (debug, info, warn, error).
// - FUI_LOG_SENSITIVE: Unredact sensitive domain fields when set to 1 or true.
// - FUI_AGENT: Enable non-interactive agent execution mode when set to 1 or true.

// Flags holds command-line flag overrides passed into Resolve.
type Flags struct {
	ConfigFile        string
	DBPath            string
	Currency          string
	Locale            string
	MonthStartDay     int
	WeekStart         string
	Theme             string
	ThemeLight        string
	ThemeDark         string
	BackupKeep        int
	ConnectorsTimeout string
	LogLevel          string
	LogSensitive      *bool
	Agent             *bool
	Verbose           bool
}

// ResolveOptions provides all inputs required to compute configuration and paths.
type ResolveOptions struct {
	HomeDir string
	Env     EnvLookup
	Flags   Flags
}

// Resolve unifies code defaults, TOML file contents, environment variables,
// and command-line flags into a validated Config instance.
//
// Precedence order (highest to lowest):
//  1. Command-line flags
//  2. FUI_* environment variables
//  3. TOML configuration file
//  4. Code defaults
func Resolve(opts ResolveOptions) (*Config, []string, error) {
	paths, err := ResolvePaths(opts.HomeDir, opts.Env)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve paths: %w", err)
	}

	cfg := Defaults()
	applyPaths(&cfg, paths, opts)

	warnings, err := loadConfigFile(&cfg, opts)
	if err != nil {
		return nil, nil, err
	}

	if err := applyEnv(&cfg, opts.Env); err != nil {
		return nil, nil, err
	}

	if err := applyFlags(&cfg, opts.Flags); err != nil {
		return nil, nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	return &cfg, warnings, nil
}

func applyPaths(cfg *Config, paths Paths, opts ResolveOptions) {
	cfg.ConfigDir = paths.ConfigDir
	cfg.ConfigFile = paths.ConfigFile
	cfg.DataDir = paths.DataDir
	cfg.DBPath = paths.DBPath
	cfg.BackupDir = paths.BackupDir
	cfg.StateDir = paths.StateDir
	cfg.LogPath = paths.LogPath
	cfg.LogLevel = DefaultLogLevel
	cfg.LogSensitive = false
	cfg.AgentMode = false

	if opts.Env != nil {
		if v := opts.Env("FUI_CONFIG"); v != "" {
			cfg.ConfigFile = v
		}
		if v := opts.Env("FUI_DB"); v != "" {
			cfg.DBPath = v
		}
	}

	if opts.Flags.ConfigFile != "" {
		cfg.ConfigFile = opts.Flags.ConfigFile
	}
	if opts.Flags.DBPath != "" {
		cfg.DBPath = opts.Flags.DBPath
	}
}

func loadConfigFile(cfg *Config, opts ResolveOptions) ([]string, error) {
	configFile := cfg.ConfigFile
	cleanPath := filepath.Clean(configFile)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// If explicitly specified by user flag or env, missing file is an error.
			explicit := opts.Flags.ConfigFile != "" || (opts.Env != nil && opts.Env("FUI_CONFIG") != "")
			if explicit {
				return nil, fmt.Errorf("config file %q: %w", configFile, err)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("read config file %q: %w", configFile, err)
	}

	var warnings []string
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if decodeErr := dec.Decode(cfg); decodeErr != nil {
		var sme *toml.StrictMissingError
		if errors.As(decodeErr, &sme) {
			for _, de := range sme.Errors {
				key := strings.Join(de.Key(), ".")
				warnings = append(warnings, fmt.Sprintf("unknown configuration key %q in %s", key, configFile))
			}
		} else {
			var de *toml.DecodeError
			if errors.As(decodeErr, &de) {
				key := strings.Join(de.Key(), ".")
				if key != "" {
					return nil, fmt.Errorf("invalid value for key %q: %w", key, de)
				}
			}
			return nil, fmt.Errorf("decode config file %q: %w", configFile, decodeErr)
		}
	}

	return warnings, nil
}

func applyEnv(cfg *Config, env EnvLookup) error {
	if env == nil {
		return nil
	}

	if v := env("FUI_CURRENCY"); v != "" {
		cfg.Currency = v
	}
	if v := env("FUI_LOCALE"); v != "" {
		cfg.Locale = v
	}
	if v := env("FUI_MONTH_START_DAY"); v != "" {
		day, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid value for key %q: %q (expected integer)", "month_start_day", v)
		}
		cfg.MonthStartDay = day
	}
	if v := env("FUI_WEEK_START"); v != "" {
		cfg.WeekStart = v
	}
	if v := env("FUI_THEME"); v != "" {
		cfg.Theme = v
	}
	if v := env("FUI_THEME_LIGHT"); v != "" {
		cfg.ThemeLight = v
	}
	if v := env("FUI_THEME_DARK"); v != "" {
		cfg.ThemeDark = v
	}
	if v := env("FUI_BACKUP_KEEP"); v != "" {
		keep, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid value for key %q: %q (expected integer)", "backup.keep", v)
		}
		cfg.Backup.Keep = keep
	}
	if err := applyEnvExtended(cfg, env); err != nil {
		return err
	}
	return nil
}

func applyEnvExtended(cfg *Config, env EnvLookup) error {
	if v := env("FUI_CONNECTORS_TIMEOUT"); v != "" {
		var d Duration
		if err := d.UnmarshalText([]byte(v)); err != nil {
			return fmt.Errorf("invalid value for key %q: %w", "connectors.timeout", err)
		}
		cfg.Connectors.Timeout = d
	}
	if v := env("FUI_LOG"); v != "" {
		cfg.LogLevel = v
	}
	if v := env("FUI_LOG_SENSITIVE"); v != "" {
		cfg.LogSensitive = parseBool(v)
	}
	if v := env("FUI_AGENT"); v != "" {
		cfg.AgentMode = parseBool(v)
	}
	return nil
}

func applyFlags(cfg *Config, flags Flags) error {
	if flags.Currency != "" {
		cfg.Currency = flags.Currency
	}
	if flags.Locale != "" {
		cfg.Locale = flags.Locale
	}
	if flags.MonthStartDay != 0 {
		cfg.MonthStartDay = flags.MonthStartDay
	}
	if flags.WeekStart != "" {
		cfg.WeekStart = flags.WeekStart
	}
	if flags.Theme != "" {
		cfg.Theme = flags.Theme
	}
	if flags.ThemeLight != "" {
		cfg.ThemeLight = flags.ThemeLight
	}
	if flags.ThemeDark != "" {
		cfg.ThemeDark = flags.ThemeDark
	}
	if flags.BackupKeep != 0 {
		cfg.Backup.Keep = flags.BackupKeep
	}
	if flags.ConnectorsTimeout != "" {
		var d Duration
		if err := d.UnmarshalText([]byte(flags.ConnectorsTimeout)); err != nil {
			return fmt.Errorf("invalid value for key %q: %w", "connectors.timeout", err)
		}
		cfg.Connectors.Timeout = d
	}
	if flags.LogLevel != "" {
		cfg.LogLevel = flags.LogLevel
	}
	if flags.Verbose {
		cfg.LogLevel = "debug"
	}
	if flags.LogSensitive != nil {
		cfg.LogSensitive = *flags.LogSensitive
	}
	if flags.Agent != nil {
		cfg.AgentMode = *flags.Agent
	}
	return nil
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
