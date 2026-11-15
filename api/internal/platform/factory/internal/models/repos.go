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
	ID             uuid.UUID
	Owner          string
	Name           string
	DefaultBranch  string
	DefaultHarness Harness
	// DefaultModel is the OpenRouter model OpenCode runs on; empty for Claude Code.
	DefaultModel      string
	SetupCommands     string
	AgentInstructions string
	TimeLimitMinutes  int
	// AutoPickup lets the runner take this repo's Ready tasks without a press of Run.
	AutoPickup bool
	// SentryProject is the Sentry project slug whose issues belong to this repo;
	// empty for none.
	SentryProject string
	// RedPRLabel and RedPRBranchPrefix scope which open PRs count as red: those
	// carrying the label or whose head branch starts with the prefix. Either
	// may be empty.
	RedPRLabel        string
	RedPRBranchPrefix string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
