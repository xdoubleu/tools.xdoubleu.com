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

// SandboxSession is one job's VM, kept across the agent runs of the job: its
// processes (a database) and files outside the mounts stay between them.
type SandboxSession interface {
	// Exec runs command with stdin as its input, returning its exit code; an error
	// means it could not run at all. The first Exec also runs the spec's setup.
	Exec(ctx context.Context, command []string, stdin string, log io.Writer) (int, error)
	// Close destroys the VM.
	Close()
}

// SessionSandbox is a Sandbox that can keep its VM between runs. One that is
// not gets a fresh VM per run.
type SessionSandbox interface {
	Sandbox
	// Open starts the job's VM from spec, without running Command.
	Open(ctx context.Context, spec SandboxSpec) (SandboxSession, error)
}

// oneShot adapts a Sandbox that only has Run: every Exec is a fresh VM.
type oneShot struct {
	box  Sandbox
	spec SandboxSpec
}

func (o oneShot) Exec(
	ctx context.Context,
	command []string,
	stdin string,
	log io.Writer,
) (int, error) {
	spec := o.spec
	spec.Command, spec.Stdin, spec.Log = command, stdin, log
	return o.box.Run(ctx, spec)
}

func (oneShot) Close() {}

// openSession keeps the job's VM when the sandbox can.
func openSession(
	ctx context.Context,
	box Sandbox,
	spec SandboxSpec,
) (SandboxSession, error) {
	if s, ok := box.(SessionSandbox); ok {
		return s.Open(ctx, spec)
	}
	return oneShot{box: box, spec: spec}, nil
}
