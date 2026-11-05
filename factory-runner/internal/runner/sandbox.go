package runner

import (
	"context"
	"io"
)

const (
	// SandboxOutcomePath is where the agent writes its outcome inside the sandbox;
	// the host reads it from SandboxSpec.OutcomeDir.
	SandboxOutcomePath = "/outcome/outcome.md"
	// SandboxCloneDir is where the job's clone is mounted inside the sandbox.
	SandboxCloneDir = "/work"
	// SandboxSessionDir is where the task's harness session state is mounted.
	SandboxSessionDir = "/session"
)

// SandboxSpec is everything one job gets: its clone, an outcome directory, its
// environment and the command to run. Nothing else of the host is reachable.
type SandboxSpec struct {
	// CloneDir is mounted read-write at SandboxCloneDir.
	CloneDir string
	// OutcomeDir is mounted read-write at the directory of SandboxOutcomePath.
	OutcomeDir string
	// SessionDir is mounted read-write at SandboxSessionDir; it outlives the job,
	// so a Reply can resume the session.
	SessionDir    string
	Env           map[string]string
	SetupCommands string
	Command       []string
	// Proxy, when set, is this job's egress proxy (it holds the job's credentials);
	// it replaces the sandbox's own.
	Proxy ProxyListener
	// Stdin is fed to Command: the prompt.
	Stdin string
	// Log receives what the command writes to stdout and stderr.
	Log io.Writer
}

// Sandbox runs one job in a fresh, isolated VM and destroys it afterwards
// (docs/adr-0043). A cancelled context stops the job.
type Sandbox interface {
	// Run returns the command's exit code; an error means it could not run at all.
	Run(ctx context.Context, spec SandboxSpec) (int, error)
}
