// Package config loads and saves the nudged config file ~/.config/nudged/config.toml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Event struct {
	Show        bool `toml:"show"`
	Sound       bool `toml:"sound"`
	HideSeconds int  `toml:"hide_seconds"` // 0 means the panel stays
}

// Events has one field per event kind; the keys match the kinds in internal/proto.
type Events struct {
	Permission Event `toml:"permission"`
	Question   Event `toml:"question"`
	Done       Event `toml:"done"`
	Waits      Event `toml:"waits"`
}

type Config struct {
	Mode           string `toml:"mode"` // "answer" or "notify"
	TimeoutMinutes int    `toml:"timeout_minutes"`
	StartAtLogin   bool   `toml:"start_at_login"`
	Events         Events `toml:"events"`
}

func Default() Config {
	return Config{
		Mode:           "answer",
		TimeoutMinutes: 30,
		StartAtLogin:   true,
		Events: Events{
			Permission: Event{Show: true},
			Question:   Event{Show: true},
			Done:       Event{Show: true, HideSeconds: 5},
			Waits:      Event{Show: true, HideSeconds: 10},
		},
	}
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "nudged", "config.toml"), nil
}

// Load reads the config file on top of Default(). It writes the defaults when the file does not exist.
func Load() (Config, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, Save(cfg)
	}
	if err != nil {
		return cfg, err
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Mode != "answer" && cfg.Mode != "notify" {
		return Default(), fmt.Errorf("%s: mode %q is not \"answer\" or \"notify\"", path, cfg.Mode)
	}
	return cfg, nil
}

// Save writes cfg to a temp file in the same folder and renames it, so a reader never sees a partial file.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*.toml") // CreateTemp uses mode 0600
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
