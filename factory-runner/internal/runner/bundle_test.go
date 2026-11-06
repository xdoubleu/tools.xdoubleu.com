package runner_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestAppBundlePath(t *testing.T) {
	t.Run("inside a bundle", func(t *testing.T) {
		exe := filepath.Join("FactoryRunner.app", "Contents", "MacOS", "factory-runner")
		assert.Equal(t, "FactoryRunner.app", runner.AppBundlePath(exe))
	})
	t.Run("bare binary", func(t *testing.T) {
		assert.Equal(t, "", runner.AppBundlePath(filepath.Join("bin", "factory-runner")))
	})
}
