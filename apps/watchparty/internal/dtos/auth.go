package dtos

import "github.com/XDoubleU/essentia/pkg/validate"

type SignInDto struct {
	Email    string `schema:"email"`
	Password string `schema:"password"`
	RoomCode string `schema:"roomcode"`
}

func (dto *SignInDto) Validate() (bool, map[string]string) {
	v := validate.New()

	validate.Check(v, "email", dto.Email, validate.IsNotEmpty)
	validate.Check(v, "password", dto.Password, validate.IsNotEmpty)

	return v.Valid(), v.Errors()
}
