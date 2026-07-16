package models

import "time"

// ProfileShare is the opaque token behind a user's public profile link
// (global.profile_shares). One share per user; regenerating replaces it.
type ProfileShare struct {
	UserID    string
	Token     string
	CreatedAt time.Time
}
