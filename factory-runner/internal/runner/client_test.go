package runner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func serve(t *testing.T, h http.HandlerFunc) *runner.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return runner.NewClient(srv.URL+"/", "1.2.3")
}

func TestPairSendsTheCodeAndReturnsTheToken(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/Pair", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Empty(t, r.Header.Get("Authorization"))
		body := make([]byte, 200)
		n, _ := r.Body.Read(body)
		assert.JSONEq(t, `{"code":"ABCDE-FGHJK","version":"1.2.3"}`, string(body[:n]))
		_, _ = w.Write([]byte(`{"deviceToken":"frt_x","runnerName":"Home Mac"}`))
	})
	token, name, err := c.Pair(context.Background(), "ABCDE-FGHJK")
	require.NoError(t, err)
	assert.Equal(t, "frt_x", token)
	assert.Equal(t, "Home Mac", name)
}

func TestPairWithoutATokenIsAnError(t *testing.T) {
	c := serve(
		t,
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) },
	)
	_, _, err := c.Pair(context.Background(), "x")
	require.Error(t, err)
}

func TestHeartbeatSendsTheBearerToken(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/Heartbeat", r.URL.Path)
		assert.Equal(t, "Bearer frt_x", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{}`))
	})
	require.NoError(t, c.Heartbeat(context.Background(), "frt_x"))
}

func TestServerErrorsKeepTheirCode(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"unauthenticated","message":"unknown device token"}`))
	})
	err := c.Heartbeat(context.Background(), "frt_x")
	require.Error(t, err)
	assert.True(t, runner.Unauthenticated(err))
	assert.Contains(t, err.Error(), "unknown device token")

	_, _, err = c.Pair(context.Background(), "x")
	assert.True(t, runner.Unauthenticated(err))
}

func TestOtherFailures(t *testing.T) {
	plain := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>"))
	})
	err := plain.Heartbeat(context.Background(), "t")
	require.Error(t, err)
	assert.False(t, runner.Unauthenticated(err))
	assert.Contains(t, err.Error(), "502")

	garbage := serve(
		t,
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("nope")) },
	)
	require.Error(t, garbage.Heartbeat(context.Background(), "t"))

	down := runner.NewClient("http://127.0.0.1:1", "v")
	require.Error(t, down.Heartbeat(context.Background(), "t"))

	bad := runner.NewClient("http://bad host", "v")
	require.Error(t, bad.Heartbeat(context.Background(), "t"))
}

func TestUnauthenticatedIgnoresOtherErrors(t *testing.T) {
	assert.False(t, runner.Unauthenticated(nil))
	assert.False(t, runner.Unauthenticated(context.Canceled))
}
