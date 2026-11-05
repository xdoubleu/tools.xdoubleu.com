package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// memSecrets is an in-memory SecretStore.
type memSecrets struct {
	values map[string]string
	setErr error
}

func newMemSecrets() *memSecrets {
	return &memSecrets{values: map[string]string{}, setErr: nil}
}

func (m *memSecrets) GetSecret(name string) (string, error) {
	if v, ok := m.values[name]; ok {
		return v, nil
	}
	return "", runner.ErrNoSecret
}

func (m *memSecrets) SetSecret(name, value string) error {
	if m.setErr != nil {
		return m.setErr
	}
	if value == "" {
		return errors.New("a secret is one non-empty line")
	}
	m.values[name] = value
	return nil
}

// claimedJob is a job with only its board account filled in.
func claimedJob(clientID, secret string) *runner.ClaimedJob {
	return &runner.ClaimedJob{
		ID: "", TaskID: "", RepoOwner: "", RepoName: "", DefaultBranch: "",
		Harness: "", Prompt: "", Branch: "", SetupCommands: "", TimeLimitMinutes: 0,
		MCPClientID: clientID, MCPClientSecret: secret, ResumeSessionID: "", Model: "",
	}
}

func withSecretInput(e env, input string) (env, *memSecrets) {
	secrets := newMemSecrets()
	e.app.secrets = secrets
	e.app.stdin = strings.NewReader(input)
	return e, secrets
}

func TestAConfiguredSecretIsReadFromStdinAndNeverEchoed(t *testing.T) {
	e, secrets := withSecretInput(newEnv(t), "  sk-ant-oat01-the-token\n")

	require.Equal(t, 0, e.do("config", "secret", "claude"))

	assert.Equal(t, "sk-ant-oat01-the-token", secrets.values["claude"])
	assert.Contains(t, e.out.String(), "Stored the claude secret")
	assert.NotContains(t, e.out.String(), "sk-ant")
	assert.NotContains(t, e.errs.String(), "sk-ant")
}

func TestASecretThatCannotBeStoredIsRefusedWithoutEchoingIt(t *testing.T) {
	for name, tc := range map[string]struct {
		args  []string
		input string
		store error
		want  string
	}{
		"empty": {
			[]string{"config", "secret", "claude"}, "  \n", nil, "non-empty",
		},
		"too long": {
			[]string{"config", "secret", "openrouter"}, strings.Repeat("a", 5000), nil,
			"longer than",
		},
		"keychain": {
			[]string{"config", "secret", "claude"}, "s3cr3t-value",
			errors.New("keychain locked"), "keychain locked",
		},
		"no name": {[]string{"config", "secret"}, "x", nil, "usage"},
		"extra":   {[]string{"config", "secret", "claude", "x"}, "x", nil, "usage"},
	} {
		e, secrets := withSecretInput(newEnv(t), tc.input)
		secrets.setErr = tc.store

		assert.Equal(t, 1, e.do(tc.args...), name)
		assert.Contains(t, e.errs.String(), tc.want, name)
		assert.NotContains(t, e.errs.String(), "s3cr3t-value", name)
		assert.Empty(t, secrets.values, name)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("stdin closed") }

func TestAnUnreadableStdinIsAnError(t *testing.T) {
	e, _ := withSecretInput(newEnv(t), "")
	e.app.stdin = brokenReader{}
	assert.Equal(t, 1, e.do("config", "secret", "claude"))
	assert.Contains(t, e.errs.String(), "stdin closed")
}

func TestTheCommitAuthorAddressIsConfigurable(t *testing.T) {
	e, _ := withSecretInput(newEnv(t), "")
	require.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "bot-email: "+runner.DefaultBotEmail+"\n")

	const bot = "123+factory[bot]@users.noreply.github.com"
	require.Equal(t, 0, e.do("config", "bot-email", bot))
	e.out.Reset()
	require.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "bot-email: "+bot)

	for _, bad := range []string{"", "no-at-sign", "a b@c.d", "a@b\nc", "<x@y.z>"} {
		assert.Equal(t, 1, e.do("config", "bot-email", bad), bad)
	}
}

func TestTheJobRunnerReadsItsCredentialsFromTheKeychainWhenAJobNeedsThem(t *testing.T) {
	e, secrets := withSecretInput(newEnv(t), "")
	cfg := runner.Config{
		Server: "https://example.com/api", Repos: nil, Image: "", DataDir: e.app.home + "/d",
		GoModCache: "", NPMCache: "", BotEmail: "bot@example.com",
	}
	jr, err := newJobRunner(context.Background(), e.app, cfg, "frt_tok", quietLog(),
		versionExec{out: "container CLI version 1.6.1", err: nil})
	require.NoError(t, err)

	assert.Equal(t, "bot@example.com", jr.AuthorEmail)
	_, err = jr.ClaudeToken()
	require.ErrorIs(t, err, runner.ErrNoSecret, "nothing stored yet")
	secrets.values["claude"] = "tok-1"
	got, err := jr.ClaudeToken()
	require.NoError(t, err)
	assert.Equal(t, "tok-1", got, "read at job time, not at startup")

	job := claimedJob("agent-1", "s")
	proxy, err := jr.Proxy(job)
	require.NoError(t, err)
	assert.NotNil(t, proxy)
	box, ok := jr.Sandbox.(*runner.ContainerSandbox)
	require.True(t, ok)
	assert.Equal(t, runner.EgressProxyPort, box.ProxyPort)
}

func proxyDo(h http.Handler, method, target string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader("{}")))
	return rec.Code
}

func TestAJobProxyOffersTheBoardOnlyToAJobWithItsOwnAccount(t *testing.T) {
	cfg := runner.Config{
		Server: "http://127.0.0.1:1", Repos: nil, Image: "", DataDir: "",
		GoModCache: "", NPMCache: "", BotEmail: "",
	}
	secrets := newMemSecrets()
	log := quietLog()

	with, err := newJobProxy(cfg,
		claimedJob("agent-1", "s"), secrets, log)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, proxyDo(with, http.MethodPost, "/mcp"),
		"the route exists; the token server is unreachable here")

	without, err := newJobProxy(cfg, claimedJob("", ""), secrets, log)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, proxyDo(without, http.MethodPost, "/mcp"),
		"no account, no board")
}

func TestAJobProxyReadsTheOpenRouterKeyOnlyWhenTheRouteIsUsed(t *testing.T) {
	cfg := runner.Config{
		Server: "http://127.0.0.1:1", Repos: nil, Image: "", DataDir: "",
		GoModCache: "", NPMCache: "", BotEmail: "",
	}
	p, err := newJobProxy(cfg, claimedJob("", ""), newMemSecrets(), quietLog())
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadGateway,
		proxyDo(p, http.MethodPost, "/openrouter/v1/chat/completions"),
		"no key stored: the request fails, the job does not")
	assert.Equal(t, http.StatusForbidden, proxyDo(p, http.MethodGet, "/other"))
	_ = io.Discard
}
