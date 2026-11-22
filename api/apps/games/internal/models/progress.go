package models

const ProgressDateFormat = "2006-01-02"

// UserIntegrations is a user's Backlog integration settings.
type UserIntegrations struct {
	UserID      string
	SteamUserID string
}
