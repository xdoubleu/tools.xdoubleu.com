// Package ward runs an agent harness in the job VM on the caller's own checkout,
// and hands the work to the next harness when one hits its usage limit.
package ward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"

	"tools.xdoubleu.com/wardcase/vm"
)

// Harness names, as typed on the command line and in Order.
const (
	Claude   = "claude"
	OpenCode = "opencode"
	Codex    = "codex"
)

// Overflow says what happens when a harness hits its usage limit.
const (
	OverflowAsk  = "ask"
	OverflowAuto = "auto"
	OverflowOff  = "off"
)

// Config is the device's wardcase settings, from ~/.config/wardcase/config.toml.
type Config struct {
	// Order is the harnesses to use, first one by default; overflow goes on to the
	// next one.
	Order []string `toml:"order"`
	// Overflow is ask, auto or off. A run without a terminal treats ask as off.
	Overflow string `toml:"overflow"`
	// Image is the job image; empty picks the newest local factory-job image.
	Image    string         `toml:"image"`
	OpenCode OpenCodeConfig `toml:"opencode"`
}

// OpenCodeConfig is OpenCode's own settings.
type OpenCodeConfig struct {
	// Model is an OpenRouter model id, "vendor/model".
	Model string `toml:"model"`
}

// ValidHarness reports whether name is a harness wardcase runs.
func ValidHarness(name string) bool {
	return name == Claude || name == OpenCode || name == Codex
}

// ConfigPath is where the device's config lives.
func ConfigPath(home string) string {
	return filepath.Join(home, ".config", "wardcase", "config.toml")
}

// LoadConfig reads the config at path; a missing file is the defaults.
func LoadConfig(path string) (Config, error) {
	cfg := Config{Order: nil, Overflow: "", Image: "", OpenCode: OpenCodeConfig{Model: ""}}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	default:
		meta, decodeErr := toml.Decode(string(data), &cfg)
		if decodeErr != nil {
			return Config{}, fmt.Errorf("parse %s: %w", path, decodeErr)
		}
		if undecoded := meta.Undecoded(); len(undecoded) > 0 {
			return Config{}, fmt.Errorf("%s: unknown setting %s", path, undecoded[0])
		}
	}
	if len(cfg.Order) == 0 {
		cfg.Order = []string{Claude, OpenCode}
	}
	if cfg.Overflow == "" {
		cfg.Overflow = OverflowAsk
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	for i, name := range c.Order {
		if !ValidHarness(name) {
			return fmt.Errorf("order: unknown harness %q (claude, opencode or codex)", name)
		}
		if slices.Index(c.Order, name) != i {
			return fmt.Errorf("order: %q is listed twice", name)
		}
	}
	if !slices.Contains([]string{OverflowAsk, OverflowAuto, OverflowOff}, c.Overflow) {
		return fmt.Errorf("overflow %q must be ask, auto or off", c.Overflow)
	}
	if c.OpenCode.Model != "" && !vm.ValidModel(c.OpenCode.Model) {
		return fmt.Errorf("opencode.model %q is not an OpenRouter model id", c.OpenCode.Model)
	}
	return nil
}

// next is the harness after name in Order, or "" if it is the last.
func (c Config) next(name string) string {
	i := slices.Index(c.Order, name)
	if i < 0 || i+1 >= len(c.Order) {
		return ""
	}
	return c.Order[i+1]
}
