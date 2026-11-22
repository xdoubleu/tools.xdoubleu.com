// Package models holds the factory's task board types.
package models

import (
	"errors"
	"slices"
	"strconv"
	"strings"
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
	// StatusTesting is merged work waiting for a person to check it.
	StatusTesting Status = "testing"
)

// Valid reports whether s is a board column.
func (s Status) Valid() bool {
	switch s {
	case StatusBacklog, StatusReady, StatusInProgress, StatusInReview,
		StatusNeedsYou, StatusDone, StatusTesting:
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

const (
	ActorHuman ActorKind = "human"
	// ActorAgent is an agent account acting through MCP; the runner comes later.
	ActorAgent ActorKind = "agent"
)

// Actor is who performs a request: a kind plus an id, and how the request
// authenticated.
type Actor struct {
	Kind ActorKind
	ID   string
	// Browser is true for a cookie session from the web app.
	Browser bool
	Admin   bool
	// Via names the job an agent account runs for, for notices; empty otherwise.
	Via string
}

// String is the "<kind>:<id>" form stored in created_by and author columns.
func (a Actor) String() string {
	return string(a.Kind) + ":" + a.ID
}

// Name is the actor for a notice: its id, and the job it acts for when it has one.
func (a Actor) Name() string {
	if a.Via == "" {
		return a.String()
	}
	return a.String() + " (" + a.Via + ")"
}

// Channel is how the actor authenticated, stored beside edits.
func (a Actor) Channel() string {
	if a.Browser {
		return "browser"
	}
	return "other"
}

// CanMoveToReady reports whether the actor may mark a task Ready: an agent
// account (its factory grant) or an admin, on any channel.
func (a Actor) CanMoveToReady() bool {
	return a.Kind == ActorAgent || a.Admin
}

// IsBrowserAdmin reports whether the actor is a human admin in a browser
// session, which running, stopping, chat, job logs and resuming pickup need
// whatever the account.
func (a Actor) IsBrowserAdmin() bool {
	return a.Kind == ActorHuman && a.Browser && a.Admin
}

// CanManageRoutines reports whether the actor may create, change, run, stop and delete
// routines: whoever may mark a task Ready.
func (a Actor) CanManageRoutines() bool {
	return a.CanMoveToReady()
}

// TaskFilter narrows a task listing; every empty field keeps all tasks.
type TaskFilter struct {
	// Query matches the title or body, ignoring case.
	Query  string
	Status Status
	RepoID *uuid.UUID
	// Label matches one of the task's labels, ignoring case.
	Label string
}

type Task struct {
	ID uuid.UUID
	// Number is the human-readable id, shown as T-<Number>.
	Number   int64
	RepoID   uuid.UUID
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
	// PRURL, PRState, PRCI and PRReview are the pull request of the task's newest
	// job that opened one, as last read from GitHub; empty when there is none.
	PRURL    string
	PRState  string
	PRCI     string
	PRReview string
	// PRMergeable is the mergeable state of PRURL, see LinkedPullRequest.
	PRMergeable string
	// PickupApproved is whether a human approved this Ready task's text for auto
	// pickup: it holds a snapshot made from a browser.
	PickupApproved bool
	// PickupBlocked is why auto pickup skipped a Ready task (bad repo settings),
	// "" while it can be picked up.
	PickupBlocked string
	// MergeSHA is the commit the task's pull request merged as; empty before a merge.
	MergeSHA string
	// Deploy is whether that commit is live, filled for the board and never stored.
	Deploy DeployState
	// DeployReason says why Deploy is DeployUnknown; empty otherwise.
	DeployReason string
}

// DeployState is whether a merged change is live in production.
type DeployState string

const (
	// DeployNone is the zero value: not merged or too old to check.
	DeployNone DeployState = ""
	// DeployUnknown is a merged task whose state couldn't be determined; the
	// task's DeployReason says why.
	DeployUnknown DeployState = "unknown"
	DeployPending DeployState = "pending"
	DeployLive    DeployState = "deployed"
)

type Comment struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	Author    string
	Body      string
	CreatedAt time.Time
}

// TaskRef formats a task number as its human-readable id.
func TaskRef(number int64) string {
	return "T-" + strconv.FormatInt(number, 10)
}

// ParseTaskRef reads "T-142", "t142" or "142" as a task number.
func ParseTaskRef(ref string) (int64, bool) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	ref = strings.TrimPrefix(strings.TrimPrefix(ref, "t"), "-")
	n, err := strconv.ParseInt(ref, 10, 64)
	return n, err == nil && n > 0
}

// ErrInvalidTaskRef is a task reference that is neither a number nor a UUID.
var ErrInvalidTaskRef = errors.New("invalid task reference")

// PickupCandidate is a Ready task as a human approved it.
type PickupCandidate struct {
	TaskID uuid.UUID
	Number int64
	RepoID uuid.UUID
	Title  string
	Body   string
}

// LabelSpike marks a task settled by the owner in an interactive session.
const LabelSpike = "spike"

// SpikeReadyMessage says why a spike never runs as a job.
const SpikeReadyMessage = "a spike is settled in an interactive session, " +
	"never as a factory job; it stays out of Ready."

// HasLabel reports whether labels holds label, ignoring case.
func HasLabel(labels []string, label string) bool {
	return slices.ContainsFunc(labels, func(l string) bool {
		return strings.EqualFold(l, label)
	})
}
