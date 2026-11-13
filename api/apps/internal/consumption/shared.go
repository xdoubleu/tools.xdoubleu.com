package consumption

import (
	"strings"

	"github.com/google/uuid"
)

const sharedPrefix = "family:"

// SharedSubject is the owner key of a family's shared list. Apps take it where
// they take a user ID, so a family reads as one person; it can't collide with
// a user ID.
func SharedSubject(familyID uuid.UUID) string {
	return sharedPrefix + familyID.String()
}

// IsSharedSubject reports whether id is a SharedSubject.
func IsSharedSubject(id string) bool {
	return strings.HasPrefix(id, sharedPrefix)
}
