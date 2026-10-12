// Package models holds the taste app's stored types.
package models

import "time"

// ProfileSections is the LLM-written profile; each list holds short bullets.
type ProfileSections struct {
	Summary  string   `json:"summary"`
	Know     []string `json:"know"`
	Like     []string `json:"like"`
	Drifting []string `json:"drifting"`
	Bounce   []string `json:"bounce"`
}

// Profile is a user's profile row. Sections is nil before the first
// generation.
type Profile struct {
	UserID      string
	Sections    *ProfileSections
	Model       string
	GeneratedAt *time.Time
	RequestedAt *time.Time
	Pending     bool
	LastError   string
}
