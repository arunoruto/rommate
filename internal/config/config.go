// Package config loads rommate's TOML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/arunoruto/rommate/internal/folders"
)

// Config is the on-disk configuration, ~/.config/rommate/config.toml.
type Config struct {
	// Server is the RomM base URL, e.g. https://romm.example.com.
	Server string `toml:"server"`
	// Token is a RomM Client API Token (rmm_...). ROMMATE_TOKEN overrides it.
	Token string `toml:"token"`
	// Profile selects the frontend layout: es-de, emudeck or retrodeck.
	Profile folders.Profile `toml:"profile"`
	// ROMDir overrides the profile's ROM root directory.
	ROMDir string `toml:"rom_dir"`
	// Folders overrides the folder for individual RomM platform slugs.
	Folders map[string]string `toml:"folders"`
	// Hints selects the TUI footer hints: auto (default), keyboard or controller.
	// auto shows controller buttons when rommate was launched through Steam.
	Hints string `toml:"hints,omitempty"`
}

// Dir returns rommate's config directory: $XDG_CONFIG_HOME/rommate, else
// ~/.config/rommate. This deliberately ignores os.UserConfigDir so macOS
// uses ~/.config too instead of ~/Library/Application Support.
func Dir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "rommate"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "rommate"), nil
}

// DataDir returns rommate's data directory (state database, lock file).
func DataDir() (string, error) {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "rommate"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "rommate"), nil
}

// Path returns the config file path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads the config file and applies environment overrides. A missing
// file is not an error; Validate reports what is missing.
func Load() (*Config, error) {
	cfg := &Config{Profile: folders.ProfileESDE}
	path, err := Path()
	if err != nil {
		return nil, err
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if v := os.Getenv("ROMMATE_SERVER"); v != "" {
		cfg.Server = v
	}
	if v := os.Getenv("ROMMATE_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("ROMMATE_ROM_DIR"); v != "" {
		cfg.ROMDir = v
	}
	if v := os.Getenv("ROMMATE_HINTS"); v != "" {
		cfg.Hints = v
	}
	if cfg.Profile == "" {
		cfg.Profile = folders.ProfileESDE
	}
	return cfg, nil
}

// Validate checks that the settings needed to talk to RomM are present.
func (c *Config) Validate() error {
	path, _ := Path()
	if c.Server == "" {
		return fmt.Errorf("no RomM server configured: set `server` in %s or ROMMATE_SERVER", path)
	}
	if c.Token == "" {
		return fmt.Errorf("no API token configured: set `token` in %s or ROMMATE_TOKEN", path)
	}
	switch c.Profile {
	case folders.ProfileESDE, folders.ProfileEmuDeck, folders.ProfileRetroDECK:
	default:
		return fmt.Errorf("unknown profile %q (want es-de, emudeck or retrodeck)", c.Profile)
	}
	switch c.Hints {
	case "", "auto", "keyboard", "controller":
	default:
		return fmt.Errorf("unknown hints %q (want auto, keyboard or controller)", c.Hints)
	}
	return nil
}

// Mapper builds the folder mapper for this config.
func (c *Config) Mapper() folders.Mapper {
	root := c.ROMDir
	if root == "" {
		root = folders.DefaultRoot(c.Profile)
	}
	return folders.Mapper{Root: root, Overrides: c.Folders}
}

// Save writes the config file with owner-only permissions (it holds a token).
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}
