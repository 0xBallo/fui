package config

import (
	"errors"
	"path/filepath"
)

// EnvLookup is an abstraction for querying environment variables without direct os.Getenv calls.
type EnvLookup func(key string) string

// Paths contains resolved filesystem locations according to XDG standards.
type Paths struct {
	ConfigDir  string
	ConfigFile string
	DataDir    string
	DBPath     string
	BackupDir  string
	StateDir   string
	LogPath    string
}

// ResolvePaths computes standard XDG directories and file paths based on homeDir and env.
// On both Linux and macOS, it falls back to:
//   - ~/.config/fui for configuration
//   - ~/.local/share/fui for data
//   - ~/.local/state/fui for state
func ResolvePaths(homeDir string, env EnvLookup) (Paths, error) {
	if homeDir == "" {
		return Paths{}, errors.New("home directory must not be empty")
	}
	if env == nil {
		env = func(string) string { return "" }
	}

	configDir := env("XDG_CONFIG_HOME")
	if configDir != "" {
		configDir = filepath.Join(configDir, "fui")
	} else {
		configDir = filepath.Join(homeDir, ".config", "fui")
	}

	dataDir := env("XDG_DATA_HOME")
	if dataDir != "" {
		dataDir = filepath.Join(dataDir, "fui")
	} else {
		dataDir = filepath.Join(homeDir, ".local", "share", "fui")
	}

	stateDir := env("XDG_STATE_HOME")
	if stateDir != "" {
		stateDir = filepath.Join(stateDir, "fui")
	} else {
		stateDir = filepath.Join(homeDir, ".local", "state", "fui")
	}

	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.toml"),
		DataDir:    dataDir,
		DBPath:     filepath.Join(dataDir, "fui.db"),
		BackupDir:  filepath.Join(dataDir, "backups"),
		StateDir:   stateDir,
		LogPath:    filepath.Join(stateDir, "fui.log"),
	}, nil
}
