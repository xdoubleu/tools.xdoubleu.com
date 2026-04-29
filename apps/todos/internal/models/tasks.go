package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusOpen     = "open"
	StatusDone     = "done"
	StatusArchived = "archived"

	LabelCategorySetup = "setup"
	LabelCategoryType  = "type"

	PriorityNone = 0
	PriorityP1   = 1
	PriorityP2   = 2
	PriorityP3   = 3
)

type Task struct {
	ID           uuid.UUID
	OwnerUserID  string
	Title        string
	Description  string
	SetupLabel   string
	TypeLabel    string
	Status       string
	Priority     int
	SortOrder    int
	CompletedAt  *time.Time
	ArchivedAt   *time.Time
	DueDate      *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SectionID    *uuid.UUID
	WorkspaceID  *uuid.UUID
	RecurDays    int
	Links        []TaskLink
	Subtasks     []Subtask
	SubtaskDone  int
	SubtaskTotal int
}

type Subtask struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	Title     string
	Done      bool
	SortOrder int
	CreatedAt time.Time
}

type TaskLink struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	URL       string
	Label     string
	SortOrder int
}

type Section struct {
	ID          uuid.UUID
	OwnerUserID string
	Name        string
	SortOrder   int
	CreatedAt   time.Time
	WorkspaceID *uuid.UUID
}

type LabelPresets struct {
	Setups []string
	Types  []string
}

type URLPattern struct {
	ID           uuid.UUID
	UserID       string
	URLPrefix    string
	PlatformName string
	TypeLabel    string
	SortOrder    int
	WorkspaceID  *uuid.UUID
}

type ArchiveSettings struct {
	UserID            string
	ArchiveAfterHours int
}

type Policy struct {
	ID                 uuid.UUID
	OwnerUserID        string
	Text               string
	ReappearAfterHours int
	SortOrder          int
	CreatedAt          time.Time
	WorkspaceID        *uuid.UUID
}

type Workspace struct {
	ID          uuid.UUID
	OwnerUserID string
	Name        string
	CreatedAt   time.Time
}

type UserSettings struct {
	UserID            string
	ActiveWorkspaceID *uuid.UUID
	ActiveWorkspace   *Workspace
}
