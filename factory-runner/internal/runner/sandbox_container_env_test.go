package runner_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEnvFileThatCannotBeWrittenFailsTheJobBeforeTheAgentRuns(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	exec := newExecScript(routesAnswer)
	box := sandboxWith(t, exec)
	box.TempDir = filepath.Join(blocker, "sub") // a directory under a file

	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "env file directory")
	assert.Empty(t, exec.find("exec"))
}
