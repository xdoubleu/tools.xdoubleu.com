package models

import "time"

// KoboStoreBook is a book from the Kobo store (bol.com) on a user's Kobo,
// identified by its store entitlement id.
type KoboStoreBook struct {
	EntitlementID string
	ISBN13        *string
	Title         string
	Authors       []string
	// Owned is nil when unknown: learned from metadata, not an entitlement.
	Owned *bool
}

// KoboStoreReading is a store book's reading state as the Kobo reports it.
type KoboStoreReading struct {
	EntitlementID string
	Percent       int
	Finished      bool
	ReadAt        *time.Time
}

// KoboStoreOutcome is what mirroring a store book's reading state did.
type KoboStoreOutcome string

const (
	// KoboStoreUnrecorded: the store book isn't recorded, nothing was mirrored.
	KoboStoreUnrecorded KoboStoreOutcome = ""
	KoboStoreMirrored   KoboStoreOutcome = "mirrored"
	KoboStoreNoMatch    KoboStoreOutcome = "no_library_match"
	KoboStoreNoProgress KoboStoreOutcome = "no_progress"
	// KoboStoreNotNewer: the library already holds a newer or further read.
	KoboStoreNotNewer KoboStoreOutcome = "not_newer"
)

// KoboStoreBookStatus is a recorded store book with its last mirror attempt
// and its current library match (nil when unmatched).
type KoboStoreBookStatus struct {
	KoboStoreBook
	LastPercent    *int
	LastReadAt     *time.Time
	LastOutcome    KoboStoreOutcome
	LastMirroredAt *time.Time
	UpdatedAt      time.Time
	Match          *UserBook
}
