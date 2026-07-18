package models

import "time"

// KoboDevice represents a registered Kobo device for a user.
type KoboDevice struct {
	ID         string
	UserID     string
	Name       string
	Serial     string // may be empty if not known
	CreatedAt  time.Time
	LastSeenAt *time.Time // nil if the device has never synced
	// LoggingEnabled reflects the in-memory debug-logging switch; it is filled
	// by the service/handler layer from KoboLogStore, not persisted in the DB.
	LoggingEnabled bool
}
