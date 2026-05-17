package dtos_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/models"
)

func TestSetAppAccessDto_Validate_AlwaysValid(t *testing.T) {
	dto := dtos.SetAppAccessDto{Grant: true}
	valid, errs := dto.Validate()
	assert.True(t, valid)
	assert.Nil(t, errs)

	dto2 := dtos.SetAppAccessDto{Grant: false}
	valid2, errs2 := dto2.Validate()
	assert.True(t, valid2)
	assert.Nil(t, errs2)
}

func TestSetRoleDto_Validate_ValidRole(t *testing.T) {
	dto := dtos.SetRoleDto{Role: models.RoleAdmin}
	valid, errs := dto.Validate()
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestSetRoleDto_Validate_ValidUserRole(t *testing.T) {
	dto := dtos.SetRoleDto{Role: models.RoleUser}
	valid, errs := dto.Validate()
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestSetRoleDto_Validate_InvalidRole(t *testing.T) {
	dto := dtos.SetRoleDto{Role: "superuser"}
	valid, errs := dto.Validate()
	assert.False(t, valid)
	assert.NotEmpty(t, errs)
}
