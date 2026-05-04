package dtos

import "github.com/xdoubleu/essentia/v4/pkg/validate"

type SignInDto struct {
	Email      string `schema:"email"`
	Password   string `schema:"password"`
	RememberMe bool   `schema:"rememberMe"`
	Redirect   string `schema:"redirect"`
}

func (dto *SignInDto) Validate() (bool, map[string]string) {
	v := validate.New()

	validate.Check(v, "email", dto.Email, validate.IsNotEmpty)
	validate.Check(v, "password", dto.Password, validate.IsNotEmpty)
	validate.Check(v, "redirect", dto.Redirect, IsRelativeURL)

	return v.Valid(), v.Errors()
}

func IsRelativeURL(url string) (bool, string) {
	if len(url) > 0 && url[0] == '/' && (len(url) == 1 || url[1] != '/') {
		return true, ""
	}
	return false, "invalid relative URL"
}

type ForgotPasswordDto struct {
	Email string `schema:"email"`
}

func (dto *ForgotPasswordDto) Validate() (bool, map[string]string) {
	v := validate.New()
	validate.Check(v, "email", dto.Email, validate.IsNotEmpty)
	return v.Valid(), v.Errors()
}
