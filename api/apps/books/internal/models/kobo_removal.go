package models

import (
	"time"

	"github.com/google/uuid"
)

// KoboRemoval is a tombstone recording that a book must be actively removed
// from the user's Kobo device on the next sync (e.g. kobo-sync was disabled,
// or the book was deleted, after it had already been synced to a device).
type KoboRemoval struct {
	BookID    uuid.UUID
	RemovedAt time.Time
}
