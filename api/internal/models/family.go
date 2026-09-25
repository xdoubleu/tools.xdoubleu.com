package models

import (
	"time"

	"github.com/google/uuid"
)

// FamilyMember is a user's family membership; no row means an implicit
// family-of-one.
type FamilyMember struct {
	UserID      string
	FamilyID    uuid.UUID
	JoinedAt    time.Time
	DisplayName string
}

// FamilyInvite is a pending invitation to join a family.
type FamilyInvite struct {
	ID         uuid.UUID
	FamilyID   uuid.UUID
	FromUserID string
	ToUserID   string
	CreatedAt  time.Time
}
