package models

import (
	"time"

	"github.com/google/uuid"
)

// FamilyMember is a user's membership row: which family they belong to and
// when they joined. A user with no row is an implicit family-of-one.
type FamilyMember struct {
	UserID   string
	FamilyID uuid.UUID
	JoinedAt time.Time
}

// FamilyInvite is a pending invitation to join a family.
type FamilyInvite struct {
	ID         uuid.UUID
	FamilyID   uuid.UUID
	FromUserID string
	ToUserID   string
	CreatedAt  time.Time
}
