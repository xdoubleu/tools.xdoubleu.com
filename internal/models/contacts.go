package models

import (
	"time"

	"github.com/google/uuid"
)

type Contact struct {
	ID            uuid.UUID
	OwnerUserID   string
	ContactUserID string
	DisplayName   string
	Status        string
	CreatedAt     time.Time
}
