package models

import (
	"time"

	"github.com/google/uuid"
)

// LinkedPullRequest is a pull request that links a task in its body, as the
// poller last read it. CI and Review are empty once it is no longer open.
type LinkedPullRequest struct {
	URL    string
	TaskID uuid.UUID
	State  string
	CI     string
	Review string
	// Mergeable is "conflicting" or "behind" for an open pull request with
	// auto-merge armed that cannot merge; UnmergeableSince is when it was first
	// read so.
	Mergeable        string
	UnmergeableSince *time.Time
	CheckedAt        time.Time
}
