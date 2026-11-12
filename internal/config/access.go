package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/0xballo/fui/internal/module"
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
	case "modules.enabled":
		return joinModuleIDs(c.Modules.Enabled), nil
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

// SetInFile loads defaults and existing file configuration from path (without applying
// environment or command-line overrides), sets and validates the given key, and saves
// the updated configuration to path.
func SetInFile(path, key, value string) error {
	if path == "" {
		return errors.New("save configuration: config file path is empty")
	}

	fileCfg := Defaults()
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	if len(data) > 0 {
		dec := toml.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if decodeErr := dec.Decode(&fileCfg); decodeErr != nil {
			var sme *toml.StrictMissingError
			if !errors.As(decodeErr, &sme) {
				return fmt.Errorf("decode config file %q: %w", path, decodeErr)
			}
		}
	}

	if err := fileCfg.applyKey(key, value); err != nil {
		return err
	}

	return fileCfg.SaveToFile(path)
}

// Set updates the value of a dotted key, validates it, persists the changes to c.ConfigFile
// without saving runtime environment or flag overrides, and updates in-memory state.
func (c *Config) Set(key, value string) error {
	if c.ConfigFile == "" {
		return errors.New("save configuration: config file path is empty")
	}
	if err := SetInFile(c.ConfigFile, key, value); err != nil {
		return err
	}
	return c.applyKey(key, value)
}

func (c *Config) applyKey(key, value string) error {
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
	return nil
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
	case "modules.enabled":
		ids, err := parseModuleIDs(value)
		if err != nil {
			return true, err
		}
		validated, err := module.Validate(ids)
		if err != nil {
			return true, fmt.Errorf("invalid value for key %q: %w", "modules.enabled", err)
		}
		if err := validateModulesEnabled(validated.IDs()); err != nil {
			return true, err
		}
		c.Modules.Enabled = validated.IDs()
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

func parseModuleIDs(value string) ([]module.ID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("invalid value for key %q: empty list", "modules.enabled")
	}
	parts := strings.Split(value, ",")
	out := make([]module.ID, 0, len(parts))
	for _, p := range parts {
		id := module.ID(strings.TrimSpace(p))
		if id == "" {
			return nil, fmt.Errorf("invalid value for key %q: empty module id", "modules.enabled")
		}
		out = append(out, id)
	}
	return out, nil
}

func joinModuleIDs(ids []module.ID) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, ",")
}

// ModulesEnabled returns the validated module set from config.
func (c *Config) ModulesEnabled() (module.Set, error) {
	return module.Validate(c.Modules.Enabled)
}

// SetModulesEnabled persists modules.enabled and updates this config in-memory.
func (c *Config) SetModulesEnabled(_ context.Context, ids []module.ID) error {
	validated, err := module.Validate(ids)
	if err != nil {
		return fmt.Errorf("invalid value for key %q: %w", "modules.enabled", err)
	}
	return c.Set("modules.enabled", joinModuleIDs(validated.IDs()))
}

// SaveToFile serializes the persistent configuration as TOML to path,
// creating directories with 0700 and writing atomically to a temporary file
// with 0600 permissions, fsyncing, and renaming over the target path.
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

	tmpFile, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file in %q: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("chmod temp config file %q: %w", tmpName, err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp config file %q: %w", tmpName, err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp config file %q: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp config file %q: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, cleanPath); err != nil {
		return fmt.Errorf("rename temp config to %q: %w", cleanPath, err)
	}
	tmpName = ""

	return nil
}

// Save writes the persistent configuration to c.ConfigFile.
func (c *Config) Save() error {
	if c.ConfigFile == "" {
		return errors.New("save configuration: config file path is empty")
	}
	return c.SaveToFile(c.ConfigFile)
}
