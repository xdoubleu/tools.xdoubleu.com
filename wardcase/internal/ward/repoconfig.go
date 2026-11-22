package ward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// repoConfigPath is the repo file, relative to the session's checkout.
const repoConfigPath = ".wardcase/config.toml"

// RepoConfig narrows what the device gives a session. It only names things the
// device configures: a repo never defines a server, URL or credential, so repo
// content (which an agent's PR can change) can't add an egress route.
type RepoConfig struct {
	// MCP lists the device's MCP servers the session gets; nil (key absent) is all.
	MCP *[]string `toml:"mcp"`
	// Plugins lists "marketplace/plugin" names; validated, not yet applied.
	Plugins *[]string `toml:"plugins"`
}

func noRepoConfig() RepoConfig { return RepoConfig{MCP: nil, Plugins: nil} }

// LoadRepoConfig reads .wardcase/config.toml of the checkout; a missing file
// narrows nothing.
func LoadRepoConfig(checkout string) (RepoConfig, error) {
	path := filepath.Join(checkout, repoConfigPath)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return noRepoConfig(), nil
	}
	if err != nil {
		return noRepoConfig(), fmt.Errorf("read %s: %w", path, err)
	}
	var cfg RepoConfig
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return noRepoConfig(), fmt.Errorf("parse %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return noRepoConfig(), fmt.Errorf("%s: %s is not allowed; a repo file only lists "+
			"names (mcp, plugins), servers belong in the device config",
			path, undecoded[0])
	}
	if cfg.Plugins != nil {
		for _, p := range *cfg.Plugins {
			if m, name, ok := strings.Cut(p, "/"); !ok || m == "" || name == "" {
				return noRepoConfig(), fmt.Errorf("%s: plugin %q must be marketplace/plugin",
					path, p)
			}
		}
	}
	return cfg, nil
}

// narrow is the configured servers the repo allows, and the listed names the
// device doesn't have.
func (c RepoConfig) narrow(servers []MCPConfig) ([]MCPConfig, []string) {
	if c.MCP == nil {
		return servers, nil
	}
	var kept []MCPConfig
	for _, s := range servers {
		for _, name := range *c.MCP {
			if name == s.Name {
				kept = append(kept, s)
				break
			}
		}
	}
	var unknown []string
	for _, name := range *c.MCP {
		found := false
		for _, s := range servers {
			found = found || s.Name == name
		}
		if !found {
			unknown = append(unknown, name)
		}
	}
	return kept, unknown
}
