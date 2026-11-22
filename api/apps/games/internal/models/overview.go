package models

import "time"

// SteamOverview is a user's library payload: the lists, backlog size,
// completion distribution and rate history over a date window.
type SteamOverview struct {
	NotStarted   []Game
	InProgress   []Game
	Completed    []Game
	Delisted     []Game
	TotalBacklog int
	Distribution []int
	CurrentRate  string
	Labels       []string
	Values       []string
	DateStart    time.Time
	DateEnd      time.Time
}

// SteamGameDetail is one game with its achievements, most commonly unlocked
// first.
type SteamGameDetail struct {
	Game         Game
	Achievements []Achievement
}
