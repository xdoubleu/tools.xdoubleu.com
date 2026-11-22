package session

import (
	"os"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

// JobMCPServers is the device's MCP servers a headless job gets: the `[[mcp]]`
// entries of ~/.config/wardcase/config.toml marked `jobs = true` that are signed in,
// as egress proxy routes, and the names of the marked ones that are not. The
// sign-ins stay in the keychain; the proxy adds the access token.
func JobMCPServers() ([]vm.MCPServer, []string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // names itself
	}
	return JobMCPServersFrom(ward.ConfigPath(home), ward.NewKeychain())
}

// JobMCPServersFrom is JobMCPServers for the config at path and the given store.
func JobMCPServersFrom(
	path string, store ward.Secrets,
) ([]vm.MCPServer, []string, error) {
	cfg, err := ward.LoadConfig(path)
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // names the path
	}
	return ward.MCPServers(cfg, store, true)
}
