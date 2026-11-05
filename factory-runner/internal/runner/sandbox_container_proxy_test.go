package runner_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheVMLearnsWhereTheProxyListens(t *testing.T) {
	var envBody string
	exec := newExecScript(routesAnswer)
	exec.onExec = func(argv []string, _ string) {
		for i, a := range argv {
			if a == "--env-file" {
				body, err := os.ReadFile(argv[i+1])
				require.NoError(t, err)
				envBody = string(body)
			}
		}
	}
	box := sandboxWith(t, exec)
	box.ProxyPort = 8118

	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.Contains(t, envBody, "FACTORY_PROXY=http://10.231.7.1:8118\n")

	envBody = ""
	box.ProxyPort = 0
	_, err = box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.False(t, strings.Contains(envBody, "FACTORY_PROXY"), envBody)
}
