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
