package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// AppName is the application directory name under user config.
	AppName = "purewin"

	// ConfigFileName is the configuration file name.
	ConfigFileName = "config.json"

	// DefaultVersion is the config schema version.
	DefaultVersion = "1"
)

// Config holds the application configuration.
type Config struct {
	// Version is the config schema version for future migrations.
	Version string `json:"version"`

	// ConfigDir is the base directory for all PureWin config files.
	ConfigDir string `json:"config_dir"`

	// CacheDir is the directory for PureWin's own cache data.
	CacheDir string `json:"cache_dir"`

	// LogFile is the path to the operations log.
	LogFile string `json:"log_file"`

	// DebugMode enables verbose debug logging.
	DebugMode bool `json:"debug_mode"`

	// DryRunMode enables dry-run globally (no actual deletions).
	DryRunMode bool `json:"dry_run_mode"`

	mu sync.RWMutex
}

// configPath returns the full path to the config.json file.
func configPath(configDir string) string {
	return filepath.Join(configDir, ConfigFileName)
}

// defaultConfigDir returns the default configuration directory using
// os.UserConfigDir() for cross-platform compatibility.
func defaultConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user config directory: %w", err)
	}
	return filepath.Join(base, AppName), nil
}

// newDefault creates a Config with sensible defaults.
func newDefault() (*Config, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return nil, err
	}

	return &Config{
		Version:    DefaultVersion,
		ConfigDir:  dir,
		CacheDir:   filepath.Join(dir, "cache"),
		LogFile:    filepath.Join(dir, "operations.log"),
		DebugMode:  false,
		DryRunMode: false,
	}, nil
}

// Load reads configuration from the standard config path.
// If the config file does not exist, it creates a default and persists it.
func Load() (*Config, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return nil, err
	}

	path := configPath(dir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config and save it.
			cfg, defErr := newDefault()
			if defErr != nil {
				return nil, defErr
			}
			if saveErr := cfg.save(path); saveErr != nil {
				return nil, fmt.Errorf("failed to write default config: %w", saveErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
	}

	// Ensure ConfigDir is set even if the file was hand-edited.
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = dir
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = filepath.Join(cfg.ConfigDir, "cache")
	}
	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join(cfg.ConfigDir, "operations.log")
	}
	if cfg.Version == "" {
		cfg.Version = DefaultVersion
	}

	return cfg, nil
}

// Save persists the current configuration to disk.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.save(configPath(c.ConfigDir))
}

// save writes the config to the given path, creating directories as needed.
func (c *Config) save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}

	return nil
}

// EnsureDirs creates the config and cache directories if they don't exist.
func (c *Config) EnsureDirs() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dirs := []string{c.ConfigDir, c.CacheDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

// SetDebug updates the debug mode and persists the change.
func (c *Config) SetDebug(enabled bool) error {
	c.mu.Lock()
	c.DebugMode = enabled
	c.mu.Unlock()
	return c.Save()
}

// SetDryRun updates the dry-run mode and persists the change.
func (c *Config) SetDryRun(enabled bool) error {
	c.mu.Lock()
	c.DryRunMode = enabled
	c.mu.Unlock()
	return c.Save()
}
