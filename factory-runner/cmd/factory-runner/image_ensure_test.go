package main

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// pullExec reports the image absent until it is pulled.
type pullExec struct{ cmds []string }

func (p *pullExec) Exec(
	_ context.Context, argv []string, _ io.Reader, _ io.Writer,
) (int, error) {
	p.cmds = append(p.cmds, argv[2])
	if argv[2] == "inspect" {
		return 1, nil
	}
	return 0, nil
}

func TestEnsureImagePullsTheBaseImageOfTheStampedVersion(t *testing.T) {
	old := WardcaseVersion
	t.Cleanup(func() { WardcaseVersion = old })
	WardcaseVersion = "1.2.3"
	x := &pullExec{cmds: nil}
	require.NoError(t, ensureImage(context.Background(), bareConfig(), x))
	assert.Equal(t, []string{"inspect", "pull"}, x.cmds)
}

func TestEnsureImageSkipsADevBuildAndAnOverride(t *testing.T) {
	old := WardcaseVersion
	t.Cleanup(func() { WardcaseVersion = old })
	x := &pullExec{cmds: nil}
	WardcaseVersion = runner.DevRelease
	require.NoError(t, ensureImage(context.Background(), bareConfig(), x))
	WardcaseVersion = "1.2.3"
	require.NoError(t, ensureImage(context.Background(), withImage("my:1"), x))
	assert.Empty(t, x.cmds)
}

func bareConfig() runner.Config {
	return runner.Config{
		Server: "", Repos: nil, Image: "", DataDir: "", GoModCache: "", NPMCache: "",
		BotEmail: "", Mode: "", MaxJobs: 0, OnLimit: "", FallbackModel: "",
	}
}

func withImage(ref string) runner.Config {
	cfg := bareConfig()
	cfg.Image = ref
	return cfg
}
