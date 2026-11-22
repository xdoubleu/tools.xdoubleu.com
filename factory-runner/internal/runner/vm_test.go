package runner_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestTheJobProxyRefusesABadUpstream(t *testing.T) {
	cfg := runner.EgressConfig{
		Server: "", MCPToken: nil, OpenRouterKey: nil, OpenAIKey: nil, GeminiKey: nil, MCPServers: nil,
		MCPRejected: nil, Transport: nil, Anthropic: "not a url", OpenRouter: "",
		OpenAI: "", Gemini: "", GoProxy: "", NPM: "", RubyGems: "",
		NixCache: "", NixSrc: "", Log: nil,
	}
	_, err := runner.NewEgressProxy(cfg)
	require.ErrorContains(t, err, "egress route anthropic")
}
