package models

import "github.com/supabase-community/gotrue-go/types"

type Scope int

const (
	AccessScope  Scope = 0
	RefreshScope Scope = 1
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	Role      Role     `json:"role"`
	AppAccess []string `json:"app_access"`
	HasMFA    bool     `json:"has_mfa"`
}

func UserFromTypesUser(user types.User) User {
	hasMFA := false
	for _, f := range user.Factors {
		if f.FactorType == "totp" && f.Status == "verified" {
			hasMFA = true
			break
		}
	}
	return User{
		ID:        user.ID.String(),
		Email:     user.Email,
		Role:      RoleUser,
		AppAccess: []string{},
		HasMFA:    hasMFA,
	}
}
