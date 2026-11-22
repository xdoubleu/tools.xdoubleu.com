package vm_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

type imgExec struct {
	have bool
	cmds []string
}

func (e *imgExec) Exec(
	_ context.Context, argv []string, _ io.Reader, _ io.Writer,
) (int, error) {
	e.cmds = append(e.cmds, strings.Join(argv[1:3], " "))
	if argv[2] == "inspect" && !e.have {
		return 1, nil
	}
	return 0, nil
}

func TestEnsureImagePullsOnlyAMissingImage(t *testing.T) {
	assert.Equal(t, "ghcr.io/xdoubleu/wardcase:1.2.3", vm.ImageRef("1.2.3"))

	have := &imgExec{have: true, cmds: nil}
	require.NoError(t, vm.EnsureImage(context.Background(), have, "i:1", io.Discard))
	assert.Equal(t, []string{"image inspect"}, have.cmds)

	missing := &imgExec{have: false, cmds: nil}
	require.NoError(t, vm.EnsureImage(context.Background(), missing, "i:1", io.Discard))
	assert.Equal(t, []string{"image inspect", "image pull"}, missing.cmds)
}
