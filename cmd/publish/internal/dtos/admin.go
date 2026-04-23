package dtos

import (
	"github.com/xdoubleu/essentia/v3/pkg/validate"
	"tools.xdoubleu.com/internal/models"
)

type SetRoleDto struct {
	Role models.Role `schema:"role"`
}

func (dto *SetRoleDto) Validate() (bool, map[string]string) {
	v := validate.New()
	validate.Check(v, "role", dto.Role, validate.IsInSlice([]models.Role{
		models.RoleAdmin,
		models.RoleUser,
	}))
	return v.Valid(), v.Errors()
}

type SetAppAccessDto struct {
	Grant bool `schema:"grant"`
}
