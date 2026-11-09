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
	// RoleService is a headless machine identity: observability tools only, no
	// app access or user data.
	RoleService Role = "service"
	// RoleAgent is an agent account: it reaches only the apps an admin granted
	// it, read or write, and never acts as a person (adr-0047).
	RoleAgent Role = "agent"
)

type User struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	Role      Role     `json:"role"`
	AppAccess []string `json:"app_access"`
	// WriteAccess is the subset of AppAccess granted at the write level; it
	// binds agents only, since a person's access is always both.
	WriteAccess []string `json:"write_access"`
	HasMFA      bool     `json:"has_mfa"`
	DisplayName string   `json:"display_name"`
}
