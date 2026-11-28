package runner_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestSystemFilesNamesTheSysctlThatFailed(t *testing.T) {
	for _, failing := range []string{"kern.num_files", "kern.maxfiles"} {
		_, _, err := runner.SystemFilesWith(func(name string) (uint32, error) {
			if name == failing {
				return 0, errors.New("denied")
			}
			return 1, nil
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), failing+": denied")
	}
}
