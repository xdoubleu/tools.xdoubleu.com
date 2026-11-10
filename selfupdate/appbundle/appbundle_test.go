package appbundle_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/selfupdate/appbundle"
)

func TestPath(t *testing.T) {
	t.Run("inside a bundle", func(t *testing.T) {
		exe := filepath.Join("Some.app", "Contents", "MacOS", "bin")
		assert.Equal(t, "Some.app", appbundle.Path(exe))
	})
	t.Run("bare binary", func(t *testing.T) {
		assert.Empty(t, appbundle.Path(filepath.Join("bin", "tool")))
	})
}

func TestResignLeavesABareBinaryAlone(t *testing.T) {
	require.NoError(
		t, appbundle.Resign(context.Background(), filepath.Join("bin", "tool")),
	)
}
