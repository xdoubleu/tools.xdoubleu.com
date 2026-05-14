package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusOpen     = "open"
	StatusDone     = "done"
	StatusArchived = "archived"

	LabelCategory = "label"

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
	Labels       []string
	Status       string
	Priority     int
	SortOrder    int
	CompletedAt  *time.Time
	ArchivedAt   *time.Time
	DueDate      *time.Time
	Deadline     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SectionID    *uuid.UUID
	WorkspaceID  *uuid.UUID
	RecurDays    int
	RecurRule    string
	Links        []TaskLink
	Subtasks     []Subtask
	SubtaskDone  int
	SubtaskTotal int
}

type Subtask struct {
	ID              uuid.UUID
	TaskID          uuid.UUID
	Title           string
	Description     string
	Done            bool
	SortOrder       int
	Priority        int
	Labels          []string
	DueDate         *time.Time
	Deadline        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ParentSubtaskID *uuid.UUID
	Children        []Subtask
}

type TaskLink struct {
	ID            uuid.UUID
	TaskID        uuid.UUID
	URL           string
	Label         string
	SortOrder     int
	ShortcutBadge string
}

type Section struct {
	ID          uuid.UUID
	OwnerUserID string
	Name        string
	SortOrder   int
	CreatedAt   time.Time
	WorkspaceID *uuid.UUID
}

type LabelPreset struct {
	Value string
	Color string
}

type LabelPresets struct {
	Labels []LabelPreset
}

func (lp *LabelPresets) ColorMap() map[string]string {
	m := make(map[string]string, len(lp.Labels))
	for _, l := range lp.Labels {
		m[l.Value] = l.Color
	}
	return m
}

func (lp *LabelPresets) Values() []string {
	vs := make([]string, len(lp.Labels))
	for i, l := range lp.Labels {
		vs[i] = l.Value
	}
	return vs
}

type URLPattern struct {
	ID           uuid.UUID
	UserID       string
	URLPrefix    string
	PlatformName string
	Label        string
	Shortcut     string
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
	HideShortcutHints bool
}
