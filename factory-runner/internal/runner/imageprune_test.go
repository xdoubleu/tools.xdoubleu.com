package runner_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

type pruneExec struct {
	images, boxes string
	listErr       error
	failDelete    bool
	deleted       []string
}

func (p *pruneExec) Exec(
	_ context.Context, argv []string, _ io.Reader, out io.Writer,
) (int, error) {
	switch {
	case p.listErr != nil:
		return 0, p.listErr
	case argv[1] == "image" && argv[2] == "list":
		_, _ = io.WriteString(out, p.images)
	case argv[1] == "ls":
		_, _ = io.WriteString(out, p.boxes)
	case argv[2] == "delete":
		if p.failDelete {
			return 1, nil
		}
		p.deleted = append(p.deleted, argv[3])
	}
	return 0, nil
}

const pruneImages = `[
 {"configuration":{"name":"ghcr.io/xdoubleu/wardcase:cur"}},
 {"configuration":{"name":"ghcr.io/xdoubleu/wardcase:old1"}},
 {"configuration":{"name":"ghcr.io/xdoubleu/wardcase:busy"}},
 {"configuration":{"name":"ghcr.io/xdoubleu/wardcase:dev"}},
 {"configuration":{"name":"docker.io/library/golang:1.27"}}]`

const pruneBoxes = `[{"configuration":{"image":{"reference":
"ghcr.io/xdoubleu/wardcase:busy"}}},
 {"configuration":{"image":{"reference":"ghcr.io/apple/builder:0.13"}}}]`

func newPrune(images, boxes string) *pruneExec {
	return &pruneExec{
		images: images, boxes: boxes, listErr: nil, failDelete: false, deleted: nil,
	}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestPruneKeepsTheCurrentReleaseAndImagesInUse(t *testing.T) {
	x := newPrune(pruneImages, pruneBoxes)
	image := "ghcr.io/xdoubleu/wardcase:cur"
	runner.PruneJobImages(context.Background(), x, image, quiet())
	assert.ElementsMatch(t, []string{
		"ghcr.io/xdoubleu/wardcase:old1",
		"ghcr.io/xdoubleu/wardcase:dev",
	}, x.deleted)
}

func TestPruneLeavesAnOverrideAlone(t *testing.T) {
	x := newPrune(pruneImages, pruneBoxes)
	image := "my-image:1"
	runner.PruneJobImages(context.Background(), x, image, quiet())
	assert.Empty(t, x.deleted)
}

func TestPruneNeverFails(t *testing.T) {
	for name, x := range map[string]*pruneExec{
		"list error": {
			images: "", boxes: "", listErr: errors.New("boom"),
			failDelete: false, deleted: nil,
		},
		"bad json": newPrune("nope", "[]"),
		"delete failed": {
			images: pruneImages, boxes: "[]", listErr: nil,
			failDelete: true, deleted: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			image := "ghcr.io/xdoubleu/wardcase:cur"
			require.NotPanics(t, func() {
				runner.PruneJobImages(context.Background(), x, image, quiet())
			})
			assert.Empty(t, x.deleted)
		})
	}
}

func TestPruneStopsWhenContainersCannotBeListed(t *testing.T) {
	x := newPrune(pruneImages, "")
	image := "ghcr.io/xdoubleu/wardcase:cur"
	runner.PruneJobImages(context.Background(), x, image, quiet())
	// An unreadable container list must not make an image look unused.
	assert.NotContains(t, strings.Join(x.deleted, ","), "busy")
}
