package mcp

import (
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/mcptools"
)

//nolint:exhaustruct // zero values are the point of these tests
func TestGetTodayRequiresUser(t *testing.T) {
	_, err := tools{}.getToday(t.Context(), mcptools.NoArgs{})
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
