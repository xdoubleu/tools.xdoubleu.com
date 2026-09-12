package models

import (
	"time"

	"github.com/google/uuid"
)

type LearningPath struct {
	ID        uuid.UUID
	UserID    string
	Title     string
	Goal      string
	Routine   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Modules   []Module
	Resources []Resource
}

type Module struct {
	ID             uuid.UUID
	LearningPathID uuid.UUID
	Title          string
	SortOrder      int
	Items          []Item
}

type Item struct {
	ID          uuid.UUID
	ModuleID    uuid.UUID
	Type        string
	Description string
	SortOrder   int
	Completed   bool
}

type Resource struct {
	ID             uuid.UUID
	LearningPathID uuid.UUID
	Text           string
	SortOrder      int
}
