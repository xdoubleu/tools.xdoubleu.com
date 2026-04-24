package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusToRead  = "to-read"
	StatusReading = "currently-reading"
	StatusRead    = "read"
	StatusDropped = "dropped"
)

type Book struct {
	ID           uuid.UUID
	Title        string
	Authors      []string
	ISBN13       *string
	ISBN10       *string
	CoverURL     *string
	Description  *string
	ExternalRefs map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserBook struct {
	ID             uuid.UUID
	UserID         string
	BookID         uuid.UUID
	Book           *Book
	Status         string
	Tags           []string
	ShelfPositions map[string]int
	Rating         *int16
	Notes          *string
	FinishedAt     []time.Time
	AddedAt        time.Time
	UpdatedAt      time.Time
}
