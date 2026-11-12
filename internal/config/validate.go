package config

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/0xballo/fui/internal/module"
)

// Validate checks whether all configuration values satisfy required constraints.
// Any invalid value results in an error explicitly naming the offending key.
func (c *Config) Validate() error {
	if err := validateCurrency(c.Currency); err != nil {
		return err
	}
	if err := validateLocale(c.Locale); err != nil {
		return err
	}
	if err := validateMonthStartDay(c.MonthStartDay); err != nil {
		return err
	}
	if err := validateWeekStart(c.WeekStart); err != nil {
		return err
	}
	if err := validateTheme(c.Theme); err != nil {
		return err
	}
	if err := validateThemeFlavor("theme_light", c.ThemeLight); err != nil {
		return err
	}
	if err := validateThemeFlavor("theme_dark", c.ThemeDark); err != nil {
		return err
	}
	if err := validateModulesEnabled(c.Modules.Enabled); err != nil {
		return err
	}
	if err := validateBudgetMode(c.Budget.Mode); err != nil {
		return err
	}
	if err := validateBackupKeep(c.Backup.Keep); err != nil {
		return err
	}
	if err := validateTimeout(c.Connectors.Timeout); err != nil {
		return err
	}
	return validateProfiles(c.Import.Profiles)
}

func validateCurrency(curr string) error {
	if len(curr) != 3 {
		return fmt.Errorf("invalid value for key %q: %q (expected 3-letter ISO code)", "currency", curr)
	}
	for _, r := range curr {
		if !unicode.IsLetter(r) || !unicode.IsUpper(r) {
			return fmt.Errorf("invalid value for key %q: %q (expected 3-letter ISO code)", "currency", curr)
		}
	}
	return nil
}

func validateLocale(loc string) error {
	switch loc {
	case LocaleIT, LocaleUS:
		return nil
	default:
		return fmt.Errorf("invalid value for key %q: %q (allowed: %s, %s)", "locale", loc, LocaleIT, LocaleUS)
	}
}

func validateMonthStartDay(day int) error {
	if day < 1 || day > 31 {
		return fmt.Errorf("invalid value for key %q: %d (must be between 1 and 31)", "month_start_day", day)
	}
	return nil
}

func validateWeekStart(ws string) error {
	switch strings.ToLower(ws) {
	case WeekStartMonday, WeekStartSunday:
		return nil
	default:
		return fmt.Errorf("invalid value for key %q: %q (allowed: %s, %s)", "week_start", ws, WeekStartMonday, WeekStartSunday)
	}
}

func validateTheme(th string) error {
	switch th {
	case ThemeAuto, ThemeLatte, ThemeFrappe, ThemeMacchiato, ThemeMocha, ThemeTerminal, ThemeHighContrast:
		return nil
	default:
		return fmt.Errorf("invalid value for key %q: %q (allowed: %s)", "theme", th, strings.Join(allThemes(), ", "))
	}
}

func validateThemeFlavor(key, value string) error {
	for _, f := range themeFlavors {
		if value == f {
			return nil
		}
	}
	return fmt.Errorf("invalid value for key %q: %q (allowed: %s)", key, value, strings.Join(themeFlavors, ", "))
}

func allThemes() []string {
	return []string{ThemeAuto, ThemeLatte, ThemeFrappe, ThemeMacchiato, ThemeMocha, ThemeTerminal, ThemeHighContrast}
}

func validateBudgetMode(mode string) error {
	switch mode {
	case BudgetModeCap, BudgetModeEnvelope:
		return nil
	default:
		return fmt.Errorf("invalid value for key %q: %q (allowed: %s, %s)", "budget.mode", mode, BudgetModeCap, BudgetModeEnvelope)
	}
}

func validateBackupKeep(keep int) error {
	if keep <= 0 {
		return fmt.Errorf("invalid value for key %q: %d (must be greater than 0)", "backup.keep", keep)
	}
	return nil
}

func validateTimeout(d Duration) error {
	if d.Duration() <= 0 {
		return fmt.Errorf("invalid value for key %q: must be positive duration", "connectors.timeout")
	}
	return nil
}

func validateModulesEnabled(ids []module.ID) error {
	if _, err := module.Validate(ids); err != nil {
		return fmt.Errorf("invalid value for key %q: %w", "modules.enabled", err)
	}
	return nil
}

func validateProfiles(profiles map[string]CSVProfile) error {
	for name, p := range profiles {
		if p.DateColumn == "" {
			return fmt.Errorf("invalid value for key %q: must not be empty", fmt.Sprintf("import.profiles.%s.date_column", name))
		}
		if p.DescriptionColumn == "" {
			return fmt.Errorf("invalid value for key %q: must not be empty", fmt.Sprintf("import.profiles.%s.description_column", name))
		}
		hasAmount := p.AmountColumn != ""
		hasDebitCredit := p.DebitColumn != "" && p.CreditColumn != ""
		if !hasAmount && !hasDebitCredit {
			return fmt.Errorf("invalid value for key %q: either amount_column or both debit_column and credit_column must be specified", fmt.Sprintf("import.profiles.%s.amount_column", name))
		}
		if p.DateFormat == "" {
			return fmt.Errorf("invalid value for key %q: must not be empty", fmt.Sprintf("import.profiles.%s.date_format", name))
		}
		if p.SkipRows < 0 {
			return fmt.Errorf("invalid value for key %q: must be non-negative", fmt.Sprintf("import.profiles.%s.skip_rows", name))
		}
	}
	return nil
}
