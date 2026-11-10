package models

import (
	"time"

	"github.com/google/uuid"
)

// LinkedPullRequest is a pull request that links a task in its body, as the
// poller last read it. CI and Review are empty once it is no longer open.
type LinkedPullRequest struct {
	URL       string
	TaskID    uuid.UUID
	State     string
	CI        string
	Review    string
	CheckedAt time.Time
}
