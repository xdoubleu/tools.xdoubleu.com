package models

import "time"

// DashboardKind identifies which public dashboard a share link belongs to
// (issue #737 — renamed from ProfileApp/"books" now that books+feeds are
// merged into one "reading" dashboard).
type DashboardKind string

const (
	DashboardKindReading DashboardKind = "reading"
	DashboardKindGames   DashboardKind = "games"
)

// ProfileShare is the opaque token behind a user's public dashboard link for
// one dashboard (global.profile_shares). One share per (user, kind);
// regenerating replaces it.
type ProfileShare struct {
	UserID    string
	App       DashboardKind
	Token     string
	CreatedAt time.Time
}
