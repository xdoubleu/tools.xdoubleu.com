package main

import (
	"context"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// menuInfo is what the menu bar needs to show and drive the runner.
type menuInfo struct {
	Control  *runner.Control
	Release  string
	BoardURL string
}

// menuFunc runs work beside the menu bar icon and returns work's error. It ends
// work (by cancelling its context) when the owner quits from the menu.
type menuFunc func(
	ctx context.Context, info menuInfo, work func(context.Context) error,
) error
