package models

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
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Role        Role     `json:"role"`
	AppAccess   []string `json:"app_access"`
	HasMFA      bool     `json:"has_mfa"`
	DisplayName string   `json:"display_name"`
}
