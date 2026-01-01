package models

import "github.com/supabase-community/gotrue-go/types"

type Scope int

const (
	AccessScope  Scope = 0
	RefreshScope Scope = 1
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func UserFromTypesUser(user types.User) User {
	return User{
		ID:    user.ID.String(),
		Email: user.Email,
	}
}
