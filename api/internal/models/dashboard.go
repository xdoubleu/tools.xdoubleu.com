package models

import "time"

// DashboardKind identifies which public dashboard a share link belongs to.
type DashboardKind string

const (
	DashboardKindReading DashboardKind = "reading"
	DashboardKindGames   DashboardKind = "games"
)

// ProfileShare is the token behind a public dashboard link; one per (user,
// kind), replaced on regeneration.
type ProfileShare struct {
	UserID    string
	App       DashboardKind
	Token     string
	CreatedAt time.Time
}
