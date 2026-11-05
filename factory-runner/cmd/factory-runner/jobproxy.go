package main

import (
	"log/slog"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// newJobProxy builds the egress proxy for one job: the board with the job's own
// account (a job without one reaches no board), OpenRouter with the stored key, and
// Anthropic with whatever the VM sends. The keys are read when a request needs them,
// so a missing one fails that request, not the whole job.
func newJobProxy(
	cfg runner.Config,
	job *runner.ClaimedJob,
	secrets runner.SecretStore,
	log *slog.Logger,
) (*runner.EgressProxy, error) {
	pc := runner.EgressConfig{
		Server:    cfg.Server,
		Transport: nil,
		OpenRouterKey: func() (string, error) {
			return secrets.GetSecret(runner.SecretOpenRouter)
		},
		MCPToken:    nil,
		MCPRejected: nil,
		Anthropic:   "",
		OpenRouter:  "",
		Log:         log,
	}
	if job.MCPClientID != "" && job.MCPClientSecret != "" {
		source := &runner.MCPTokenSource{
			ServerURL:    cfg.Server,
			ClientID:     job.MCPClientID,
			ClientSecret: job.MCPClientSecret,
			HTTP:         nil,
			Now:          nil,
		}
		pc.MCPToken = source.Token
		pc.MCPRejected = source.Invalidate
	}
	return runner.NewEgressProxy(pc)
}
