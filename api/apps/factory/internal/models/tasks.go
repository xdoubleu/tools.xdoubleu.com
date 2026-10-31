// Package models holds the factory's task board types.
package models

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in_progress"
	StatusInReview   Status = "in_review"
	StatusNeedsYou   Status = "needs_you"
	StatusDone       Status = "done"
)

// Valid reports whether s is a board column.
func (s Status) Valid() bool {
	switch s {
	case StatusBacklog, StatusReady, StatusInProgress, StatusInReview,
		StatusNeedsYou, StatusDone:
		return true
	}
	return false
}

type Priority string

const (
	PriorityP0 Priority = "p0"
	PriorityP1 Priority = "p1"
	PriorityP2 Priority = "p2"
	// PriorityInvalid stands for an unrecognised value; Valid rejects it.
	PriorityInvalid Priority = "invalid"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	return p == PriorityP0 || p == PriorityP1 || p == PriorityP2
}

type ActorKind string

// ActorHuman is the only kind so far; agents and runners come later.
const ActorHuman ActorKind = "human"

// Actor is who performs a request: a kind plus an id, and how the request
// authenticated.
type Actor struct {
	Kind ActorKind
	ID   string
	// Browser is true for a cookie session from the web app.
	Browser bool
	Admin   bool
}

// String is the "<kind>:<id>" form stored in created_by and author columns.
func (a Actor) String() string {
	return string(a.Kind) + ":" + a.ID
}

// Channel is how the actor authenticated, stored beside edits.
func (a Actor) Channel() string {
	if a.Browser {
		return "browser"
	}
	return "other"
}

// CanMoveToReady reports whether the actor may start a task: a human admin in
// a browser session.
func (a Actor) CanMoveToReady() bool {
	return a.Kind == ActorHuman && a.Browser && a.Admin
}

type Task struct {
	ID       uuid.UUID
	Title    string
	Body     string
	Status   Status
	Priority Priority
	Labels   []string
	// CreatedBy is the creating actor as "<kind>:<id>".
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CommentCount int
}

type Comment struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	Author    string
	Body      string
	CreatedAt time.Time
}
