package runner_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestOSExecerRunsAHostCommandAndReportsItsExitCode(t *testing.T) {
	var out bytes.Buffer
	code, err := runner.OSExecer{}.Exec(
		context.Background(),
		[]string{"sh", "-c", "cat; echo err >&2; exit 3"},
		strings.NewReader("in\n"),
		&out,
	)
	require.NoError(t, err)
	assert.Equal(t, 3, code)
	assert.Equal(t, "in\nerr\n", out.String())
}

func TestOSExecerErrorsWhenTheCommandCannotRunAtAll(t *testing.T) {
	code, err := runner.OSExecer{}.Exec(
		context.Background(), []string{"no-such-binary-xyz"}, nil, io.Discard)
	require.ErrorContains(t, err, "no-such-binary-xyz")
	assert.Equal(t, -1, code)
}

func TestAJobIDIsReducedToASafeVMName(t *testing.T) {
	exec := newExecScript(routesAnswer)
	s := spec(t, io.Discard)
	s.Env["FACTORY_JOB"] = "../../Job/X"
	_, err := sandboxWith(t, exec).Run(context.Background(), s)
	require.NoError(t, err)
	assert.Equal(t, "factory-job-jobx", exec.find("run")[0][5])

	exec = newExecScript(routesAnswer)
	delete(s.Env, "FACTORY_JOB")
	_, err = sandboxWith(t, exec).Run(context.Background(), s)
	require.NoError(t, err)
	assert.Equal(t, "factory-job-x", exec.find("run")[0][5])
}

func TestRoutesThatAreNotNetworksAreIgnored(t *testing.T) {
	routes := "default 1.2.3.4 UG en0\nnot.an.ip x\n300.1.1.1 x\n10.231.7/bad x\n" +
		"1.2.3.4.5 x\n\n"
	assert.Equal(t, []string{"10.231.7.0/24"}, runner.FreeSubnets(routes, 1))
}
