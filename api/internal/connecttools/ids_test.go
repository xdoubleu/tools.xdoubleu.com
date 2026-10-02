package connecttools_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/connecttools"
)

func TestParseClientID_Empty(t *testing.T) {
	id, err := connecttools.ParseClientID("")
	require.NoError(t, err)
	assert.False(t, id.Valid)
}

func TestParseClientID_Valid(t *testing.T) {
	want := uuid.New()
	id, err := connecttools.ParseClientID(want.String())
	require.NoError(t, err)
	assert.Equal(t, uuid.NullUUID{UUID: want, Valid: true}, id)
}

func TestParseClientID_Invalid(t *testing.T) {
	_, err := connecttools.ParseClientID("nope")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
