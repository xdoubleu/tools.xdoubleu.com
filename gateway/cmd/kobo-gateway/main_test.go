package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInvalidFlag(t *testing.T) {
	var out strings.Builder

	err := run([]string{"--no-such-flag"}, &out)

	assert.Error(t, err)
}

func TestRunUnknownCommand(t *testing.T) {
	var out strings.Builder

	err := run([]string{"frobnicate"}, &out)

	assert.ErrorContains(t, err, `unknown command "frobnicate"`)
}

func TestRunUpdateCommandFailure(t *testing.T) {
	// The origin serves HTML instead of a Mach-O binary, so the update
	// fails safely before anything replaces the running executable.
	downloads := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>not a binary</html>"))
		},
	))
	defer downloads.Close()

	var out strings.Builder

	err := run([]string{"--origin", downloads.URL, "update"}, &out)

	assert.ErrorContains(t, err, "not a valid gateway binary")
	assert.Contains(t, out.String(), "downloading latest gateway")
}

func TestRunServeFailsOnOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := addr.Port

	var out strings.Builder

	err = run([]string{
		"--port", fmt.Sprint(port),
		"--volumes-root", t.TempDir(),
		"--allow-origin", "https://one.example",
		"--allow-origin", "https://two.example",
	}, &out)

	assert.Error(t, err)
	assert.Contains(t, out.String(), "listening on https://127.0.0.1:")
}
