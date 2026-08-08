package gateway

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testConfig() Config {
	//nolint:exhaustruct //only the fields under test are needed
	return Config{
		APIBinPath:      "api-bin",
		APIPort:         8001,
		WebNodeBin:      "node",
		WebServerJS:     "unused",
		WebPort:         3000,
		APIURL:          "http://localhost:8000",
		SupabaseURL:     "http://supabase.local",
		SupabaseAnonKey: "anon-key",
		Release:         "test-release",
	}
}

func TestAPIProcessEnv_OverridesPortAndKeepsRest(t *testing.T) {
	t.Setenv("PORT", "8000")
	t.Setenv("SOME_SECRET", "shh")

	env := apiProcessEnv(testConfig())

	assert.Contains(t, env, "PORT=8001")
	assert.Contains(t, env, "SOME_SECRET=shh")
	for _, e := range env {
		assert.NotContains(t, e, "PORT=8000", "gateway's own PORT must not leak to the api child")
	}
}

func TestNewAPICmd_UsesAPIBinPath(t *testing.T) {
	cmd := NewAPICmd(context.Background(), testConfig())

	assert.Equal(t, "api-bin", cmd.Args[0])
	assert.Equal(t, os.Stdout, cmd.Stdout)
}

func TestWebProcessEnv(t *testing.T) {
	env := webProcessEnv(testConfig())

	assert.Contains(t, env, "PORT=3000")
	assert.Contains(t, env, "RELEASE=test-release")
	assert.Contains(t, env, "API_URL=http://localhost:8000")
	assert.Contains(t, env, "SUPABASE_URL=http://supabase.local")
	assert.Contains(t, env, "SUPABASE_ANON_KEY=anon-key")
}

func TestNewWebCmd_UsesWebNodeBinAndServerJS(t *testing.T) {
	cmd := NewWebCmd(context.Background(), testConfig())

	assert.Equal(t, []string{"node", "unused"}, cmd.Args)
}
