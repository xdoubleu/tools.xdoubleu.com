package sharing_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/sharing"
)

func TestValidateShareTarget_RejectsEmptyTarget(t *testing.T) {
	err := sharing.ValidateShareTarget("owner", "")
	require.Error(t, err)

	var httpErr *iapp.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}

func TestValidateShareTarget_RejectsSelfShare(t *testing.T) {
	err := sharing.ValidateShareTarget("owner", "owner")
	require.Error(t, err)

	var httpErr *iapp.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}

func TestValidateShareTarget_AllowsDistinctTarget(t *testing.T) {
	assert.NoError(t, sharing.ValidateShareTarget("owner", "friend"))
}
