package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ReadingSourceWeb    = "web"
	ReadingSourceKobo   = "kobo"
	ReadingSourceManual = "manual"
)

type BookReadingState struct {
	UserID    string
	BookID    uuid.UUID
	Source    string
	Percent   int
	Location  *string
	UpdatedAt time.Time
}
