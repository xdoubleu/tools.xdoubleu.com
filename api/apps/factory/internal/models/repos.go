package models

import (
	"time"

	"github.com/google/uuid"
)

type Harness string

const (
	HarnessClaudeCode Harness = "claude_code"
	HarnessOpenCode   Harness = "opencode"
	// HarnessInvalid stands for an unrecognised value; Valid rejects it.
	HarnessInvalid Harness = "invalid"
)

// Valid reports whether h is a known harness.
func (h Harness) Valid() bool {
	return h == HarnessClaudeCode || h == HarnessOpenCode
}

// Repo is a GitHub repository the factory runs jobs for, with the settings
// those jobs need.
type Repo struct {
	ID                uuid.UUID
	Owner             string
	Name              string
	DefaultBranch     string
	DefaultHarness    Harness
	SetupCommands     string
	AgentInstructions string
	TimeLimitMinutes  int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
