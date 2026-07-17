package models

import "time"

// ProfileApp identifies which app a profile share link belongs to.
type ProfileApp string

const (
	ProfileAppBooks ProfileApp = "books"
	ProfileAppGames ProfileApp = "games"
)

// ProfileShare is the opaque token behind a user's public profile link for
// one app (global.profile_shares). One share per (user, app); regenerating
// replaces it.
type ProfileShare struct {
	UserID    string
	App       ProfileApp
	Token     string
	CreatedAt time.Time
}
