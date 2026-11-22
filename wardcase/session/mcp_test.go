package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/session"
)

type memStore map[string]string

func (m memStore) Get(name string) (string, error) {
	if v, ok := m[name]; ok {
		return v, nil
	}
	return "", ward.ErrNoSecret
}

func (m memStore) Set(name, value string) error { m[name] = value; return nil }

func (m memStore) Delete(name string) error { delete(m, name); return nil }

const deviceConfig = `
[[mcp]]
name = "posthog"
url = "https://mcp.example.com/posthog"
jobs = true

[[mcp]]
name = "private"
url = "https://mcp.example.com/private"

[[mcp]]
name = "linear"
url = "https://mcp.example.com/linear"
jobs = true
`

func TestJobsGetOnlyTheSignedInServersMarkedForJobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(deviceConfig), 0o600))
	store := memStore{}
	for _, name := range []string{"posthog", "private"} {
		require.NoError(t, ward.SaveMCP(store, name, ward.MCPCreds{
			ClientID: "id", ClientSecret: "", Resource: "",
			TokenURL:     "https://mcp.example.com/token",
			RefreshToken: "refresh-" + name,
		}))
	}
	before := len(store)

	servers, missing, err := session.JobMCPServersFrom(path, store)
	require.NoError(t, err)

	require.Len(t, servers, 1)
	assert.Equal(t, "posthog", servers[0].Name)
	assert.Equal(t, []string{"linear"}, missing, "marked for jobs, not signed in")
	assert.Len(t, store, before, "reading writes nothing")
}

func TestAnInvalidDeviceConfigIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("bogus = 1\n"), 0o600))

	_, _, err := session.JobMCPServersFrom(path, memStore{})
	require.ErrorContains(t, err, "unknown setting")
}

func TestNoDeviceConfigMeansNoServers(t *testing.T) {
	servers, missing, err := session.JobMCPServersFrom(
		filepath.Join(t.TempDir(), "none.toml"), memStore{})
	require.NoError(t, err)
	assert.Empty(t, servers)
	assert.Empty(t, missing)
}

func TestTheDeviceServersAreReadFromTheHomeConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	servers, missing, err := session.JobMCPServers()
	require.NoError(t, err)
	assert.Empty(t, servers)
	assert.Empty(t, missing)
}

func TestWithoutAHomeDirectoryTheDeviceServersAreAnError(t *testing.T) {
	t.Setenv("HOME", "")

	_, _, err := session.JobMCPServers()
	require.Error(t, err)
}
