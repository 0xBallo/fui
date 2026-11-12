package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/module"
)

func TestDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfg, warnings, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("Resolve defaults failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	if cfg.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", cfg.Currency)
	}
	if cfg.Locale != "it-IT" {
		t.Errorf("locale = %q, want it-IT", cfg.Locale)
	}
	if cfg.MonthStartDay != 1 {
		t.Errorf("month_start_day = %d, want 1", cfg.MonthStartDay)
	}
	if cfg.WeekStart != "monday" {
		t.Errorf("week_start = %q, want monday", cfg.WeekStart)
	}
	if cfg.Theme != "auto" {
		t.Errorf("theme = %q, want auto", cfg.Theme)
	}
	if cfg.ThemeLight != "latte" {
		t.Errorf("theme_light = %q, want latte", cfg.ThemeLight)
	}
	if cfg.ThemeDark != "mocha" {
		t.Errorf("theme_dark = %q, want mocha", cfg.ThemeDark)
	}
	if cfg.Backup.Keep != 10 {
		t.Errorf("backup.keep = %d, want 10", cfg.Backup.Keep)
	}
	if cfg.Connectors.Timeout.Duration() != 120*time.Second {
		t.Errorf("connectors.timeout = %v, want 120s", cfg.Connectors.Timeout)
	}
	if len(cfg.Import.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %d", len(cfg.Import.Profiles))
	}
	if diff := cmp.Diff(module.Default().IDs(), cfg.Modules.Enabled); diff != "" {
		t.Errorf("modules.enabled mismatch (-want +got):\n%s", diff)
	}
}

func TestPrecedenceLevels(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "fui")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tomlFile := filepath.Join(configDir, "config.toml")

	// 1. Level: File overrides defaults
	tomlContent := `
currency = "USD"
locale = "en-US"
month_start_day = 15
week_start = "sunday"
theme = "high-contrast"

[backup]
keep = 20

[connectors]
timeout = "60s"
`
	if err := os.WriteFile(tomlFile, []byte(tomlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfgFile, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("Resolve with file failed: %v", err)
	}
	if cfgFile.Currency != "USD" || cfgFile.Backup.Keep != 20 || cfgFile.MonthStartDay != 15 {
		t.Errorf("file level resolution failed: %+v", cfgFile)
	}

	// 2. Level: Environment variables override file and defaults
	envMap := map[string]string{
		"FUI_CURRENCY":        "GBP",
		"FUI_MONTH_START_DAY": "5",
		"FUI_BACKUP_KEEP":     "30",
		"FUI_LOG":             "debug",
		"FUI_AGENT":           "1",
		"FUI_LOG_SENSITIVE":   "true",
	}
	envLookup := func(k string) string { return envMap[k] }

	cfgEnv, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
		Env:     envLookup,
	})
	if err != nil {
		t.Fatalf("Resolve with env failed: %v", err)
	}
	if cfgEnv.Currency != "GBP" {
		t.Errorf("currency = %q, want GBP (from env)", cfgEnv.Currency)
	}
	if cfgEnv.MonthStartDay != 5 {
		t.Errorf("month_start_day = %d, want 5 (from env)", cfgEnv.MonthStartDay)
	}
	if cfgEnv.Backup.Keep != 30 {
		t.Errorf("backup.keep = %d, want 30 (from env)", cfgEnv.Backup.Keep)
	}
	if cfgEnv.LogLevel != "debug" || !cfgEnv.AgentMode || !cfgEnv.LogSensitive {
		t.Errorf("env flags failed: log=%s, agent=%v, sens=%v", cfgEnv.LogLevel, cfgEnv.AgentMode, cfgEnv.LogSensitive)
	}

	// 3. Level: Flags override environment variables, file, and defaults
	agentFalse := false
	sensFalse := false
	cfgFlags, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
		Env:     envLookup,
		Flags: config.Flags{
			Currency:      "CHF",
			MonthStartDay: 10,
			BackupKeep:    5,
			Agent:         &agentFalse,
			LogSensitive:  &sensFalse,
			Verbose:       true,
		},
	})
	if err != nil {
		t.Fatalf("Resolve with flags failed: %v", err)
	}
	if cfgFlags.Currency != "CHF" {
		t.Errorf("currency = %q, want CHF (from flags)", cfgFlags.Currency)
	}
	if cfgFlags.MonthStartDay != 10 {
		t.Errorf("month_start_day = %d, want 10 (from flags)", cfgFlags.MonthStartDay)
	}
	if cfgFlags.Backup.Keep != 5 {
		t.Errorf("backup.keep = %d, want 5 (from flags)", cfgFlags.Backup.Keep)
	}
	if cfgFlags.AgentMode {
		t.Errorf("agentMode should be false from flags override")
	}
	if cfgFlags.LogSensitive {
		t.Errorf("logSensitive should be false from flags override")
	}
	if cfgFlags.LogLevel != "debug" {
		t.Errorf("logLevel = %q, want debug from verbose flag", cfgFlags.LogLevel)
	}
}

func TestStrictDecodingWarnings(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "fui")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tomlFile := filepath.Join(configDir, "config.toml")

	tomlContent := `
currency = "EUR"
unknown_field = "ignore_me"
another_unknown = 42

[backup]
keep = 10
extra_backup_opt = true
`
	if err := os.WriteFile(tomlFile, []byte(tomlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, warnings, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("Resolve failed on unknown keys: %v", err)
	}

	if cfg.Currency != "EUR" || cfg.Backup.Keep != 10 {
		t.Errorf("known fields should be preserved, got %+v", cfg)
	}

	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d: %v", len(warnings), warnings)
	}

	joined := strings.Join(warnings, " ")
	for _, expectedKey := range []string{"unknown_field", "another_unknown", "backup.extra_backup_opt"} {
		if !strings.Contains(joined, expectedKey) {
			t.Errorf("warnings did not mention expected key %q: %v", expectedKey, warnings)
		}
	}
}

func TestInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr string
	}{
		{
			name:    "invalid currency length",
			toml:    `currency = "EURO"`,
			wantErr: `"currency"`,
		},
		{
			name:    "invalid currency lowercase",
			toml:    `currency = "eur"`,
			wantErr: `"currency"`,
		},
		{
			name:    "invalid locale",
			toml:    `locale = "de-DE"`,
			wantErr: `"locale"`,
		},
		{
			name:    "invalid month_start_day zero",
			toml:    `month_start_day = 0`,
			wantErr: `"month_start_day"`,
		},
		{
			name:    "invalid month_start_day above 31",
			toml:    `month_start_day = 32`,
			wantErr: `"month_start_day"`,
		},
		{
			name:    "invalid week_start",
			toml:    `week_start = "wednesday"`,
			wantErr: `"week_start"`,
		},
		{
			name:    "invalid theme",
			toml:    `theme = "solarized"`,
			wantErr: `"theme"`,
		},
		{
			name:    "invalid theme_light",
			toml:    `theme_light = "terminal"`,
			wantErr: `"theme_light"`,
		},
		{
			name:    "invalid theme_dark",
			toml:    `theme_dark = "auto"`,
			wantErr: `"theme_dark"`,
		},
		{
			name:    "invalid backup.keep zero",
			toml:    `[backup]` + "\n" + `keep = 0`,
			wantErr: `"backup.keep"`,
		},
		{
			name:    "invalid timeout negative",
			toml:    `[connectors]` + "\n" + `timeout = "-10s"`,
			wantErr: `"connectors.timeout"`,
		},
		{
			name:    "modules missing core",
			toml:    `[modules]` + "\n" + `enabled = ["budget"]`,
			wantErr: `"modules.enabled"`,
		},
		{
			name:    "modules missing dependency",
			toml:    `[modules]` + "\n" + `enabled = ["core", "crypto"]`,
			wantErr: `"modules.enabled"`,
		},
		{
			name:    "modules unknown id",
			toml:    `[modules]` + "\n" + `enabled = ["core", "unknown"]`,
			wantErr: `"modules.enabled"`,
		},
		{
			name: "profile missing date_column",
			toml: `
[import.profiles.test]
description_column = "Desc"
amount_column = "Amount"
date_format = "2006-01-02"
`,
			wantErr: `"import.profiles.test.date_column"`,
		},
		{
			name: "profile missing amount and debit/credit",
			toml: `
[import.profiles.test]
date_column = "Date"
description_column = "Desc"
date_format = "2006-01-02"
`,
			wantErr: `"import.profiles.test.amount_column"`,
		},
		{
			name: "profile missing date_format",
			toml: `
[import.profiles.test]
date_column = "Date"
description_column = "Desc"
amount_column = "Amount"
`,
			wantErr: `"import.profiles.test.date_format"`,
		},
		{
			name: "profile negative skip_rows",
			toml: `
[import.profiles.test]
date_column = "Date"
description_column = "Desc"
amount_column = "Amount"
date_format = "2006-01-02"
skip_rows = -1
`,
			wantErr: `"import.profiles.test.skip_rows"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			configDir := filepath.Join(tmp, ".config", "fui")
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				t.Fatal(err)
			}
			tomlFile := filepath.Join(configDir, "config.toml")
			if err := os.WriteFile(tomlFile, []byte(tt.toml), 0o600); err != nil {
				t.Fatal(err)
			}

			_, _, err := config.Resolve(config.ResolveOptions{
				HomeDir: tmp,
			})
			if err == nil {
				t.Fatalf("expected error containing %s, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain expected key %s", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestXDGPathsAndOverrides(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "userhome")

	// Standard fallback paths
	paths, err := config.ResolvePaths(homeDir, nil)
	if err != nil {
		t.Fatalf("ResolvePaths failed: %v", err)
	}

	wantConfig := filepath.Join(homeDir, ".config", "fui")
	wantData := filepath.Join(homeDir, ".local", "share", "fui")
	wantState := filepath.Join(homeDir, ".local", "state", "fui")

	if paths.ConfigDir != wantConfig {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, wantConfig)
	}
	if paths.DataDir != wantData {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, wantData)
	}
	if paths.StateDir != wantState {
		t.Errorf("StateDir = %q, want %q", paths.StateDir, wantState)
	}
	if paths.DBPath != filepath.Join(wantData, "fui.db") {
		t.Errorf("DBPath = %q, want %q", paths.DBPath, filepath.Join(wantData, "fui.db"))
	}
	if paths.BackupDir != filepath.Join(wantData, "backups") {
		t.Errorf("BackupDir = %q, want %q", paths.BackupDir, filepath.Join(wantData, "backups"))
	}
	if paths.LogPath != filepath.Join(wantState, "fui.log") {
		t.Errorf("LogPath = %q, want %q", paths.LogPath, filepath.Join(wantState, "fui.log"))
	}

	// Overrides via XDG environment variables
	customConfig := filepath.Join(tmp, "custom_config")
	customData := filepath.Join(tmp, "custom_data")
	customState := filepath.Join(tmp, "custom_state")

	xdgEnv := func(k string) string {
		switch k {
		case "XDG_CONFIG_HOME":
			return customConfig
		case "XDG_DATA_HOME":
			return customData
		case "XDG_STATE_HOME":
			return customState
		default:
			return ""
		}
	}

	customPaths, err := config.ResolvePaths(homeDir, xdgEnv)
	if err != nil {
		t.Fatalf("ResolvePaths with XDG failed: %v", err)
	}
	if customPaths.ConfigDir != filepath.Join(customConfig, "fui") {
		t.Errorf("custom ConfigDir = %q", customPaths.ConfigDir)
	}
	if customPaths.DataDir != filepath.Join(customData, "fui") {
		t.Errorf("custom DataDir = %q", customPaths.DataDir)
	}
	if customPaths.StateDir != filepath.Join(customState, "fui") {
		t.Errorf("custom StateDir = %q", customPaths.StateDir)
	}

	// Test Resolve with explicit FUI_DB and flags
	cfg, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: homeDir,
		Env: func(k string) string {
			if k == "FUI_DB" {
				return "/custom/env/fui.db"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBPath != "/custom/env/fui.db" {
		t.Errorf("DBPath = %q, want /custom/env/fui.db", cfg.DBPath)
	}

	cfgFlag, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: homeDir,
		Flags: config.Flags{
			DBPath: "/custom/flag/fui.db",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfgFlag.DBPath != "/custom/flag/fui.db" {
		t.Errorf("DBPath = %q, want /custom/flag/fui.db", cfgFlag.DBPath)
	}
}

func TestGetSetAndPersistence(t *testing.T) {
	tmp := t.TempDir()
	cfg, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Verify Get for default keys
	val, err := cfg.Get("currency")
	if err != nil || val != "EUR" {
		t.Fatalf("Get(currency) = %q, err %v", val, err)
	}
	val, err = cfg.Get("backup.keep")
	if err != nil || val != "10" {
		t.Fatalf("Get(backup.keep) = %q, err %v", val, err)
	}
	val, err = cfg.Get("modules.enabled")
	if err != nil || val != "analytics,budget,core,recurring" {
		t.Fatalf("Get(modules.enabled) = %q, err %v", val, err)
	}

	// Set valid keys and verify persistence
	if err := cfg.Set("currency", "USD"); err != nil {
		t.Fatalf("Set(currency, USD) failed: %v", err)
	}
	if err := cfg.Set("locale", "en-US"); err != nil {
		t.Fatalf("Set(locale, en-US) failed: %v", err)
	}
	if err := cfg.Set("month_start_day", "25"); err != nil {
		t.Fatalf("Set(month_start_day) failed: %v", err)
	}
	if err := cfg.Set("week_start", "sunday"); err != nil {
		t.Fatalf("Set(week_start) failed: %v", err)
	}
	if err := cfg.Set("theme", "high-contrast"); err != nil {
		t.Fatalf("Set(theme) failed: %v", err)
	}
	if err := cfg.Set("theme_light", "frappe"); err != nil {
		t.Fatalf("Set(theme_light) failed: %v", err)
	}
	if err := cfg.Set("theme_dark", "macchiato"); err != nil {
		t.Fatalf("Set(theme_dark) failed: %v", err)
	}
	if err := cfg.Set("backup.keep", "15"); err != nil {
		t.Fatalf("Set(backup.keep) failed: %v", err)
	}
	if err := cfg.Set("connectors.timeout", "45s"); err != nil {
		t.Fatalf("Set(connectors.timeout) failed: %v", err)
	}
	if err := cfg.Set("modules.enabled", "core,budget,recurring,simulations,ai"); err != nil {
		t.Fatalf("Set(modules.enabled) failed: %v", err)
	}

	// Set profile fields
	fieldsToSet := map[string]string{
		"date_column":        "Date",
		"amount_column":      "Amount",
		"debit_column":       "Debit",
		"credit_column":      "Credit",
		"description_column": "Details",
		"external_id_column": "ExtID",
		"target_account":     "Checking",
		"delimiter":          ";",
		"encoding":           "utf-8",
		"date_format":        "2006-01-02",
		"decimal_separator":  ",",
		"skip_rows":          "2",
	}

	for f, val := range fieldsToSet {
		key := "import.profiles.bank." + f
		if err := cfg.Set(key, val); err != nil {
			t.Fatalf("Set(%s, %s) failed: %v", key, val, err)
		}
		got, err := cfg.Get(key)
		if err != nil || got != val {
			t.Fatalf("Get(%s) = %q, err %v", key, got, err)
		}
	}

	// Verify file permissions 0600 and dir 0700
	fileInfo, err := os.Stat(cfg.ConfigFile)
	if err != nil {
		t.Fatalf("stat config file failed: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file perms = %#o, want 0600", perm)
	}

	dirInfo, err := os.Stat(filepath.Dir(cfg.ConfigFile))
	if err != nil {
		t.Fatalf("stat config dir failed: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir perms = %#o, want 0700", perm)
	}

	// Reload config from disk to prove persistence
	reloaded, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("reloading config failed: %v", err)
	}

	if reloaded.Currency != "USD" {
		t.Errorf("reloaded currency = %q, want USD", reloaded.Currency)
	}
	if reloaded.Locale != "en-US" {
		t.Errorf("reloaded locale = %q, want en-US", reloaded.Locale)
	}
	if reloaded.MonthStartDay != 25 {
		t.Errorf("reloaded month_start_day = %d, want 25", reloaded.MonthStartDay)
	}
	if diff := cmp.Diff(
		[]module.ID{module.AI, module.Budget, module.Core, module.Recurring, module.Simulations},
		reloaded.Modules.Enabled,
	); diff != "" {
		t.Errorf("reloaded modules.enabled mismatch (-want +got):\n%s", diff)
	}
	if reloaded.WeekStart != "sunday" {
		t.Errorf("reloaded week_start = %q, want sunday", reloaded.WeekStart)
	}
	if reloaded.Theme != "high-contrast" {
		t.Errorf("reloaded theme = %q, want high-contrast", reloaded.Theme)
	}
	if reloaded.ThemeLight != "frappe" {
		t.Errorf("reloaded theme_light = %q, want frappe", reloaded.ThemeLight)
	}
	if reloaded.ThemeDark != "macchiato" {
		t.Errorf("reloaded theme_dark = %q, want macchiato", reloaded.ThemeDark)
	}
	if reloaded.Backup.Keep != 15 {
		t.Errorf("reloaded backup.keep = %d, want 15", reloaded.Backup.Keep)
	}
	if reloaded.Connectors.Timeout.Duration() != 45*time.Second {
		t.Errorf("reloaded timeout = %v, want 45s", reloaded.Connectors.Timeout)
	}
	p := reloaded.Import.Profiles["bank"]
	if p.DateColumn != "Date" || p.AmountColumn != "Amount" || p.Delimiter != ";" {
		t.Errorf("reloaded profile mismatch: %+v", p)
	}

	// Test Set with invalid value returns error naming key
	if err := cfg.Set("locale", "fr-FR"); err == nil {
		t.Error("expected error setting invalid locale, got nil")
	} else if !strings.Contains(err.Error(), `"locale"`) {
		t.Errorf("error %q should name key locale", err.Error())
	}

	// Test Get and Set unknown key
	if _, err := cfg.Get("unknown.key"); err == nil {
		t.Error("expected error getting unknown key, got nil")
	}
	if err := cfg.Set("unknown.key", "val"); err == nil {
		t.Error("expected error setting unknown key, got nil")
	}
}

func TestDurationParsing(t *testing.T) {
	var d config.Duration
	if err := d.UnmarshalText([]byte("90s")); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(90*time.Second, d.Duration()); diff != "" {
		t.Errorf("unexpected duration (-want +got):\n%s", diff)
	}
	if d.String() != "1m30s" {
		t.Errorf("d.String() = %q, want 1m30s", d.String())
	}

	if err := d.UnmarshalText([]byte("120")); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(120*time.Second, d.Duration()); diff != "" {
		t.Errorf("unexpected integer duration (-want +got):\n%s", diff)
	}

	if err := d.UnmarshalText([]byte("")); err != nil || d.Duration() != 0 {
		t.Errorf("unmarshal empty text failed: %v", err)
	}

	if err := d.UnmarshalText([]byte("notaduration")); err == nil {
		t.Error("expected error for invalid duration string, got nil")
	}

	text, err := d.MarshalText()
	if err != nil || string(text) != "0s" {
		t.Errorf("MarshalText = %s, err %v", text, err)
	}
}

func TestResolveErrorsAndEdgeCases(t *testing.T) {
	tmp := t.TempDir()

	// Missing home dir
	if _, _, err := config.Resolve(config.ResolveOptions{}); err == nil {
		t.Error("expected error for empty home dir, got nil")
	}

	// Explicit config file that does not exist
	_, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
		Flags: config.Flags{
			ConfigFile: filepath.Join(tmp, "does_not_exist.toml"),
		},
	})
	if err == nil {
		t.Error("expected error for missing explicit config file, got nil")
	}

	// Explicit config file via env that does not exist
	_, _, err = config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
		Env: func(k string) string {
			if k == "FUI_CONFIG" {
				return filepath.Join(tmp, "missing_via_env.toml")
			}
			return ""
		},
	})
	if err == nil {
		t.Error("expected error for missing explicit config file via env, got nil")
	}

	// Malformed TOML file
	badFile := filepath.Join(tmp, "bad.toml")
	if err := os.WriteFile(badFile, []byte("this is not toml [[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
		Flags: config.Flags{
			ConfigFile: badFile,
		},
	})
	if err == nil {
		t.Error("expected error for malformed TOML file, got nil")
	}

	// Invalid integer in env vars
	badEnvs := []struct {
		key string
		val string
	}{
		{"FUI_MONTH_START_DAY", "bad"},
		{"FUI_BACKUP_KEEP", "bad"},
		{"FUI_CONNECTORS_TIMEOUT", "bad"},
	}
	for _, be := range badEnvs {
		_, _, err := config.Resolve(config.ResolveOptions{
			HomeDir: tmp,
			Env: func(k string) string {
				if k == be.key {
					return be.val
				}
				return ""
			},
		})
		if err == nil {
			t.Errorf("expected error for invalid env %s=%s, got nil", be.key, be.val)
		}
	}

	// SaveToFile empty path, Save empty ConfigFile
	var emptyCfg config.Config
	if err := emptyCfg.Save(); err == nil {
		t.Error("expected error saving with empty ConfigFile, got nil")
	}
	if err := emptyCfg.SaveToFile(""); err == nil {
		t.Error("expected error saving with empty path, got nil")
	}

	// Profile error cases on Get and Set
	cfg := config.Defaults()
	if _, err := cfg.Get("import.profiles.nonexistent.date_column"); err == nil {
		t.Error("expected error getting field of nonexistent profile, got nil")
	}
	if _, err := cfg.Get("import.profiles.foo.invalid_field"); err == nil {
		t.Error("expected error getting invalid profile field, got nil")
	}
	if _, err := cfg.Get("import.profiles.too.short"); err == nil {
		t.Error("expected error for malformed profile key on Get, got nil")
	}
	if err := cfg.Set("import.profiles.too.short", "val"); err == nil {
		t.Error("expected error for malformed profile key on Set, got nil")
	}
	if err := cfg.Set("import.profiles.p.invalid_field", "val"); err == nil {
		t.Error("expected error for unknown profile field on Set, got nil")
	}
	if err := cfg.Set("import.profiles.p.skip_rows", "not_a_number"); err == nil {
		t.Error("expected error for invalid skip_rows integer on Set, got nil")
	}
	if err := cfg.Set("import.profiles.p.skip_rows", "-5"); err == nil {
		t.Error("expected error for negative skip_rows on Set, got nil")
	}
	if err := cfg.Set("backup.keep", "not_a_number"); err == nil {
		t.Error("expected error for invalid backup.keep integer, got nil")
	}
	if err := cfg.Set("month_start_day", "not_a_number"); err == nil {
		t.Error("expected error for invalid month_start_day integer, got nil")
	}
	if err := cfg.Set("connectors.timeout", "bad_dur"); err == nil {
		t.Error("expected error for invalid connectors.timeout, got nil")
	}
	if err := cfg.Set("modules.enabled", "budget"); err == nil {
		t.Error("expected error for invalid modules.enabled, got nil")
	}
}

func TestSetDoesNotPersistEnvOverrides(t *testing.T) {
	tmp := t.TempDir()

	env := func(k string) string {
		if k == "FUI_CURRENCY" {
			return "USD"
		}
		return ""
	}

	cfg, _, err := config.Resolve(config.ResolveOptions{
		HomeDir: tmp,
		Env:     env,
	})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if cfg.Currency != "USD" {
		t.Fatalf("cfg.Currency = %q, want USD", cfg.Currency)
	}

	// Set a different key: locale
	if err := cfg.Set("locale", "en-US"); err != nil {
		t.Fatalf("Set(locale) failed: %v", err)
	}

	// Read file directly from disk
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}

	fileContent := string(data)
	if strings.Contains(fileContent, "USD") {
		t.Errorf("config file persisted env override USD:\n%s", fileContent)
	}
}

func TestSetModulesEnabled(t *testing.T) {
	tmp := t.TempDir()
	cfg, _, err := config.Resolve(config.ResolveOptions{HomeDir: tmp})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	want := []module.ID{module.Budget, module.Core, module.Recurring}
	if err := cfg.SetModulesEnabled(t.Context(), want); err != nil {
		t.Fatalf("SetModulesEnabled failed: %v", err)
	}

	got, err := cfg.ModulesEnabled()
	if err != nil {
		t.Fatalf("ModulesEnabled failed: %v", err)
	}
	if diff := cmp.Diff(want, got.IDs()); diff != "" {
		t.Fatalf("ModulesEnabled mismatch (-want +got):\n%s", diff)
	}
}

func TestSaveToFileAtomic(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")

	// Write initial file
	if err := os.WriteFile(configPath, []byte("currency = \"EUR\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	infoBefore, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	cfg.Currency = "USD"
	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	infoAfter, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}

	// Atomic rename replaces the inode; an in-place truncate reuses the same inode.
	if os.SameFile(infoBefore, infoAfter) {
		t.Errorf("SaveToFile truncated in place instead of atomically renaming over target")
	}

	// Check permissions on the atomically renamed file
	if perm := infoAfter.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected permissions 0600, got %#o", perm)
	}

	// Ensure no leftover temporary files in dir
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.toml" {
		t.Errorf("unexpected directory contents after atomic save: %v", entries)
	}
}
