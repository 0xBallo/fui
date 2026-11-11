package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Get returns the string representation of the configuration value for a dotted key.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "currency":
		return c.Currency, nil
	case "locale":
		return c.Locale, nil
	case "month_start_day":
		return strconv.Itoa(c.MonthStartDay), nil
	case "week_start":
		return c.WeekStart, nil
	case "theme":
		return c.Theme, nil
	case "backup.keep":
		return strconv.Itoa(c.Backup.Keep), nil
	case "connectors.timeout":
		return c.Connectors.Timeout.String(), nil
	default:
		if strings.HasPrefix(key, "import.profiles.") {
			return c.getProfileField(key)
		}
		return "", fmt.Errorf("unknown configuration key: %q", key)
	}
}

func (c *Config) getProfileField(key string) (string, error) {
	parts := strings.Split(key, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid profile key format: %q", key)
	}
	name, field := parts[2], parts[3]
	p, ok := c.Import.Profiles[name]
	if !ok {
		return "", fmt.Errorf("profile %q not found", name)
	}

	if val, found := getProfileColumn(&p, field); found {
		return val, nil
	}
	if val, found := getProfileFormat(&p, field); found {
		return val, nil
	}
	return "", fmt.Errorf("unknown profile field %q in key %q", field, key)
}

func getProfileColumn(p *CSVProfile, field string) (string, bool) {
	switch field {
	case "date_column":
		return p.DateColumn, true
	case "amount_column":
		return p.AmountColumn, true
	case "debit_column":
		return p.DebitColumn, true
	case "credit_column":
		return p.CreditColumn, true
	case "description_column":
		return p.DescriptionColumn, true
	case "external_id_column":
		return p.ExternalIDColumn, true
	case "target_account":
		return p.TargetAccount, true
	default:
		return "", false
	}
}

func getProfileFormat(p *CSVProfile, field string) (string, bool) {
	switch field {
	case "delimiter":
		return p.Delimiter, true
	case "encoding":
		return p.Encoding, true
	case "date_format":
		return p.DateFormat, true
	case "decimal_separator":
		return p.DecimalSeparator, true
	case "skip_rows":
		return strconv.Itoa(p.SkipRows), true
	default:
		return "", false
	}
}

// Set updates the value of a dotted key, validates it, and persists the changes to c.ConfigFile.
func (c *Config) Set(key, value string) error {
	handled, err := c.setScalarField(key, value)
	if err != nil {
		return err
	}
	if !handled {
		handled, err = c.setComplexField(key, value)
		if err != nil {
			return err
		}
	}
	if !handled {
		if strings.HasPrefix(key, "import.profiles.") {
			if err := c.setProfileField(key, value); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unknown configuration key: %q", key)
		}
	}

	return c.Save()
}

func (c *Config) setScalarField(key, value string) (bool, error) {
	switch key {
	case "currency":
		if err := validateCurrency(value); err != nil {
			return true, err
		}
		c.Currency = value
		return true, nil
	case "locale":
		if err := validateLocale(value); err != nil {
			return true, err
		}
		c.Locale = value
		return true, nil
	case "month_start_day":
		day, err := strconv.Atoi(value)
		if err != nil {
			return true, fmt.Errorf("invalid value for key %q: %q (expected integer)", key, value)
		}
		if err := validateMonthStartDay(day); err != nil {
			return true, err
		}
		c.MonthStartDay = day
		return true, nil
	case "week_start":
		if err := validateWeekStart(value); err != nil {
			return true, err
		}
		c.WeekStart = strings.ToLower(value)
		return true, nil
	case "theme":
		if err := validateTheme(value); err != nil {
			return true, err
		}
		c.Theme = strings.ToLower(value)
		return true, nil
	default:
		return false, nil
	}
}

func (c *Config) setComplexField(key, value string) (bool, error) {
	switch key {
	case "backup.keep":
		keep, err := strconv.Atoi(value)
		if err != nil {
			return true, fmt.Errorf("invalid value for key %q: %q (expected integer)", key, value)
		}
		if err := validateBackupKeep(keep); err != nil {
			return true, err
		}
		c.Backup.Keep = keep
		return true, nil
	case "connectors.timeout":
		var d Duration
		if err := d.UnmarshalText([]byte(value)); err != nil {
			return true, fmt.Errorf("invalid value for key %q: %w", key, err)
		}
		if err := validateTimeout(d); err != nil {
			return true, err
		}
		c.Connectors.Timeout = d
		return true, nil
	default:
		return false, nil
	}
}

func (c *Config) setProfileField(key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) != 4 {
		return fmt.Errorf("invalid profile key format: %q", key)
	}
	name, field := parts[2], parts[3]
	if c.Import.Profiles == nil {
		c.Import.Profiles = make(map[string]CSVProfile)
	}
	p := c.Import.Profiles[name]

	if setProfileColumn(&p, field, value) {
		c.Import.Profiles[name] = p
		return nil
	}

	handled, err := setProfileFormat(&p, key, field, value)
	if err != nil {
		return err
	}
	if !handled {
		return fmt.Errorf("unknown profile field %q in key %q", field, key)
	}

	c.Import.Profiles[name] = p
	return nil
}

func setProfileColumn(p *CSVProfile, field, value string) bool {
	switch field {
	case "date_column":
		p.DateColumn = value
		return true
	case "amount_column":
		p.AmountColumn = value
		return true
	case "debit_column":
		p.DebitColumn = value
		return true
	case "credit_column":
		p.CreditColumn = value
		return true
	case "description_column":
		p.DescriptionColumn = value
		return true
	case "external_id_column":
		p.ExternalIDColumn = value
		return true
	case "target_account":
		p.TargetAccount = value
		return true
	default:
		return false
	}
}

func setProfileFormat(p *CSVProfile, key, field, value string) (bool, error) {
	switch field {
	case "delimiter":
		p.Delimiter = value
		return true, nil
	case "encoding":
		p.Encoding = value
		return true, nil
	case "date_format":
		p.DateFormat = value
		return true, nil
	case "decimal_separator":
		p.DecimalSeparator = value
		return true, nil
	case "skip_rows":
		rows, err := strconv.Atoi(value)
		if err != nil {
			return true, fmt.Errorf("invalid value for key %q: %q (expected integer)", key, value)
		}
		if rows < 0 {
			return true, fmt.Errorf("invalid value for key %q: must be non-negative", key)
		}
		p.SkipRows = rows
		return true, nil
	default:
		return false, nil
	}
}

// SaveToFile serializes the persistent configuration as TOML to path,
// creating directories with 0700 and files with 0600 permissions.
func (c *Config) SaveToFile(path string) error {
	if path == "" {
		return errors.New("save configuration: path must not be empty")
	}
	cleanPath := filepath.Clean(path)
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory %q: %w", dir, err)
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config TOML: %w", err)
	}

	f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open config file %q: %w", cleanPath, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write config file %q: %w", cleanPath, err)
	}
	return nil
}

// Save writes the persistent configuration to c.ConfigFile.
func (c *Config) Save() error {
	if c.ConfigFile == "" {
		return errors.New("save configuration: config file path is empty")
	}
	return c.SaveToFile(c.ConfigFile)
}
