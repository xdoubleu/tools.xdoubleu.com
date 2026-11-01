package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// DefaultServer is the API root the runner talks to.
	DefaultServer = "https://tools.xdoubleu.com/api"

	dirPerm  = 0o700
	filePerm = 0o600
)

// Config is the runner's own, local trust: the server can't change it.
type Config struct {
	Server string   `json:"server"`
	Repos  []string `json:"repos"`
}

// ConfigPath is where the config lives under homeDir.
func ConfigPath(homeDir string) string {
	return filepath.Join(
		homeDir,
		"Library",
		"Application Support",
		"factory-runner",
		"config.json",
	)
}

// LoadConfig reads the config; a missing file is the default one.
func LoadConfig(path string) (Config, error) {
	cfg := Config{Server: DefaultServer, Repos: []string{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server == "" {
		cfg.Server = DefaultServer
	}
	return cfg, nil
}

// Save writes the config, owner-only.
func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err = os.WriteFile(path, append(data, '\n'), filePerm); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// AddRepo allows a repo; adding one twice is a no-op.
func (c *Config) AddRepo(repo string) error {
	if !ValidRepo(repo) {
		return fmt.Errorf("%q is not owner/name", repo)
	}
	for _, r := range c.Repos {
		if strings.EqualFold(r, repo) {
			return nil
		}
	}
	c.Repos = append(c.Repos, repo)
	slices.Sort(c.Repos)
	return nil
}

// RemoveRepo drops a repo from the allowlist and reports whether it was there.
func (c *Config) RemoveRepo(repo string) bool {
	before := len(c.Repos)
	c.Repos = slices.DeleteFunc(c.Repos, func(r string) bool {
		return strings.EqualFold(r, repo)
	})
	return len(c.Repos) < before
}

// SetServer points the runner at another API root. Only https is accepted,
// except for loopback so it can be tried against a local api.
func (c *Config) SetServer(raw string) error {
	if !secureServer(raw) {
		return fmt.Errorf("server %q must be an https URL (http only for localhost)", raw)
	}
	c.Server = strings.TrimRight(raw, "/")
	return nil
}

func secureServer(raw string) bool {
	if strings.HasPrefix(raw, "https://") && len(raw) > len("https://") {
		return true
	}
	for _, host := range []string{"localhost", "127.0.0.1"} {
		if raw == "http://"+host || strings.HasPrefix(raw, "http://"+host+":") ||
			strings.HasPrefix(raw, "http://"+host+"/") {
			return true
		}
	}
	return false
}
