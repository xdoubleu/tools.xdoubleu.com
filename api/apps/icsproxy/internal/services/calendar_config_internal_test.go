package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

func TestAuthorizeConfigAccess_NotFound(t *testing.T) {
	//nolint:exhaustruct // test fields only
	err := authorizeConfigAccess(models.FilterConfig{}, false, "user-1")
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestAuthorizeConfigAccess_WrongOwner(t *testing.T) {
	//nolint:exhaustruct // test fields only
	cfg := models.FilterConfig{UserID: "owner"}
	err := authorizeConfigAccess(cfg, true, "someone-else")
	assert.ErrorIs(t, err, ErrConfigAccessDenied)
}

func TestAuthorizeConfigAccess_Owner(t *testing.T) {
	//nolint:exhaustruct // test fields only
	cfg := models.FilterConfig{UserID: "owner"}
	err := authorizeConfigAccess(cfg, true, "owner")
	assert.NoError(t, err)
}
