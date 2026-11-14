package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

type listExec struct{ calls int }

func (l *listExec) Exec(
	_ context.Context, _ []string, _ io.Reader, out io.Writer,
) (int, error) {
	l.calls++
	_, _ = io.WriteString(out, "[]")
	return 0, nil
}

func TestPruneImagesRunsOnlyForTheImplicitReleaseImage(t *testing.T) {
	old := Release
	t.Cleanup(func() { Release = old })
	cfg := runner.Config{
		Server: "", Repos: nil, Image: "", DataDir: "", GoModCache: "", NPMCache: "",
		BotEmail: "", Mode: "", MaxJobs: 0,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	Release = "abc"
	x := &listExec{calls: 0}
	pruneImages(context.Background(), cfg, x, log)
	assert.Positive(t, x.calls)

	Release = runner.DevRelease
	x = &listExec{calls: 0}
	pruneImages(context.Background(), cfg, x, log)
	assert.Zero(t, x.calls)
}
