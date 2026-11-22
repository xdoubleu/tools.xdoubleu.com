package vm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ImageRepo is where each wardcase release publishes its base image.
const ImageRepo = "ghcr.io/xdoubleu/wardcase"

// ImageRef is the base image of one release.
func ImageRef(version string) string { return ImageRepo + ":" + version }

// EnsureImage pulls image unless the container CLI already has it.
func EnsureImage(
	ctx context.Context, execer Execer, image string, notes io.Writer,
) error {
	var out bytes.Buffer
	code, err := execer.Exec(ctx, []string{ContainerCLI, "image", "inspect", image},
		nil, &out)
	if err == nil && code == 0 {
		return nil
	}
	fmt.Fprintf(notes, "wardcase: pulling %s\n", image)
	out.Reset()
	code, err = execer.Exec(ctx, []string{ContainerCLI, "image", "pull", image},
		nil, &out)
	if err != nil || code != 0 {
		return fmt.Errorf("pull %s: %w", image, errors.Join(err,
			fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out.String()))))
	}
	return nil
}
