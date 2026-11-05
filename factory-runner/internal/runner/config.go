package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	// DefaultServer is the API root the runner talks to.
	DefaultServer = "https://tools.xdoubleu.com/api"

	// DefaultImage is the job image `make image` builds.
	DefaultImage = "factory-job:dev"

	dirPerm  = 0o700
	filePerm = 0o600
)

// imagePattern is an image reference that cannot be read as a flag.
var imagePattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`,
)

// Config is the runner's own, local trust: the server can't change it.
type Config struct {
	Server string   `json:"server"`
	Repos  []string `json:"repos"`
	// Image is the job image; empty means DefaultImage.
	Image string `json:"image,omitempty"`
	// DataDir holds the mirrors, clones and sessions; empty means DefaultDataDir.
	DataDir string `json:"dataDir,omitempty"`
	// GoModCache and NPMCache are mounted read-only into the VM, which has no
	// registry access; empty means the job gets none.
	GoModCache string `json:"goModCache,omitempty"`
	NPMCache   string `json:"npmCache,omitempty"`
}

// JobImage is the image jobs run in.
func (c Config) JobImage() string {
	if c.Image == "" {
		return DefaultImage
	}
	return c.Image
}

// DefaultDataDir is where job data lives under homeDir.
func DefaultDataDir(homeDir string) string {
	return filepath.Join(homeDir, "Library", "Application Support", "factory-runner",
		"data")
}

// DataPath is the data directory, or the default under homeDir.
func (c Config) DataPath(homeDir string) string {
	if c.DataDir == "" {
		return DefaultDataDir(homeDir)
	}
	return c.DataDir
}

// SetImage chooses the job image.
func (c *Config) SetImage(ref string) error {
	if !imagePattern.MatchString(ref) {
		return fmt.Errorf("%q is not an image reference", ref)
	}
	c.Image = ref
	return nil
}

// SetDir sets one of the directories: "data", "gocache" or "npmcache". It must be
// an absolute path the VM's mount option can carry.
func (c *Config) SetDir(which, dir string) error {
	if !filepath.IsAbs(dir) || strings.ContainsAny(dir, ",\n\r") {
		return fmt.Errorf("%q must be an absolute path without commas", dir)
	}
	dir = filepath.Clean(dir)
	switch which {
	case "data":
		c.DataDir = dir
	case "gocache":
		c.GoModCache = dir
	case "npmcache":
		c.NPMCache = dir
	default:
		return fmt.Errorf("unknown directory %q", which)
	}
	return nil
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
	cfg := Config{
		Server:     DefaultServer,
		Repos:      []string{},
		Image:      "",
		DataDir:    "",
		GoModCache: "",
		NPMCache:   "",
	}
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
