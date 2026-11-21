package runner

import (
	"context"

	"tools.xdoubleu.com/wardcase/vm"
)

// The job VM, its egress proxy and the harness adapters live in wardcase
// (docs/adr-0055); these names keep the runner's own.

type (
	Sandbox          = vm.Sandbox
	SandboxSession   = vm.SandboxSession
	SessionSandbox   = vm.SessionSandbox
	SandboxSpec      = vm.SandboxSpec
	ContainerSandbox = vm.ContainerSandbox
	ProxyListener    = vm.ProxyListener
	Execer           = vm.Execer
	OSExecer         = vm.OSExecer
	EgressConfig     = vm.EgressConfig
	EgressProxy      = vm.EgressProxy
	Harness          = vm.Harness
	ClaudeCode       = vm.ClaudeCode
	OpenCode         = vm.OpenCode
)

const (
	SandboxOutcomePath = vm.SandboxOutcomePath
	SandboxCloneDir    = vm.SandboxCloneDir
	SandboxSessionDir  = vm.SandboxSessionDir
	ContainerVersion   = vm.ContainerVersion
	EgressProxyPort    = vm.EgressProxyPort
	HarnessClaudeCode  = vm.HarnessClaudeCode
	HarnessOpenCode    = vm.HarnessOpenCode
	ClaudeConfigSeed   = vm.ClaudeConfigSeed

	containerCLI = vm.ContainerCLI
)

// NewEgressProxy builds the proxy for one job.
func NewEgressProxy(cfg EgressConfig) (*EgressProxy, error) { return vm.NewEgressProxy(cfg) }

func validModel(model string) bool { return vm.ValidModel(model) }

func openSession(ctx context.Context, box Sandbox, spec SandboxSpec) (SandboxSession, error) {
	return vm.OpenSession(ctx, box, spec)
}
