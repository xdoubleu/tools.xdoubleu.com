package models

// SteamTypeID keys Steam completion-rate rows in the games.progress table.
// The value must stay stable across deploys (rows were written under the
// former backlog schema with this same type ID).
const SteamTypeID string = "0"
