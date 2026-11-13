package main

import (
	"log/slog"
	"sync"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// cachedKey reads a stored secret on first use and keeps it for the job: a model
// call is many requests, and each read would start a `security` process. A failed
// read is not kept, so a key stored mid-job is picked up.
func cachedKey(secrets runner.SecretStore, name string) func() (string, error) {
	var mu sync.Mutex
	var key string
	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if key != "" {
			return key, nil
		}
		value, err := secrets.GetSecret(name)
		if err != nil {
			return "", err
		}
		key = value
		return key, nil
	}
}

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
		Server:        cfg.Server,
		Transport:     nil,
		OpenRouterKey: cachedKey(secrets, runner.SecretOpenRouter),
		MCPToken:      nil,
		MCPRejected:   nil,
		Anthropic:     "",
		OpenRouter:    "", GoProxy: "", NPM: "",
		Log: log,
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
