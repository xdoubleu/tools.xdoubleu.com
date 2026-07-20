package reading

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPOptStr(t *testing.T) {
	assert.Nil(t, mcpOptStr(""))

	got := mcpOptStr("x")
	require.NotNil(t, got)
	assert.Equal(t, "x", *got)
}
