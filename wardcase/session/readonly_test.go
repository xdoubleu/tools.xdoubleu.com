package session //nolint:testpackage // readOnly is unexported

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

func TestASessionNeverStoresOrDeletesASecret(t *testing.T) {
	require.ErrorIs(t, readOnly{nil}.Set("a", "b"), ward.ErrNoSecret)
	require.ErrorIs(t, readOnly{nil}.Delete("a"), ward.ErrNoSecret)
}
